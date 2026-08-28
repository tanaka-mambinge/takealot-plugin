package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/t12e/takealot-cli/internal/config"
)

const (
	MobileAPIBase   = "https://api.takealot.com/rest/v-1-16-0"
	MobileUserAgent = "TAL-Android/4.2.1 (fi.android.takealot; build:800749; 16; samsung; SM-A356E; Phone)"
)

type OTPProvider func(context.Context) (string, bool, error)

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter string
}

func (e *APIError) Error() string {
	message := fmt.Sprintf("Takealot mobile API error (%s, HTTP %d): %s", e.Code, e.StatusCode, e.Message)
	if e.RetryAfter != "" {
		message += "; retry-after: " + e.RetryAfter
	}
	return message
}

type Manager struct {
	httpClient *http.Client
	base       string
	store      *Store
	jar        http.CookieJar
	mu         sync.Mutex
}

func NewManager(httpClient *http.Client, base string, store *Store) *Manager {
	if httpClient == nil {
		httpClient = config.NewHTTPClient()
	}
	if store == nil {
		store = NewStore()
	}
	jar, _ := cookiejar.New(nil)
	clientCopy := *httpClient
	clientCopy.Jar = jar
	return &Manager{httpClient: &clientCopy, base: strings.TrimRight(firstNonEmpty(base, MobileAPIBase), "/"), store: store, jar: jar}
}

func (m *Manager) Login(ctx context.Context, email, password string, otp OTPProvider) (Session, error) {
	if strings.TrimSpace(email) == "" || password == "" {
		return Session{}, errors.New("email and password are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	credentials := []map[string]any{
		{"field_id": "email", "value": email},
		{"field_id": "password", "value": password},
		{"field_id": "captcha", "value": ""},
	}
	payload := map[string]any{"platform": "android", "sections": []any{
		map[string]any{"section_id": "customer_login", "fields": credentials},
	}}
	response, raw, err := m.post(ctx, "/customers/login", payload, false)
	if err != nil {
		return Session{}, err
	}
	if session, ok := sessionFromResponse(response, m.jar, m.base); ok {
		if err := m.store.Save(session); err != nil {
			return Session{}, fmt.Errorf("save Takealot session: %w", err)
		}
		return session, nil
	}
	if !hasTwoStepChallenge(response) {
		return Session{}, loginResponseError(raw)
	}
	if otp == nil {
		return Session{}, errors.New("Takealot requires a one-time password; run login again with OTP input")
	}
	code, trust, err := otp(ctx)
	if err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(code) == "" {
		return Session{}, errors.New("one-time password cannot be empty")
	}
	payload["sections"] = []any{
		map[string]any{"section_id": "customer_login", "fields": credentials},
		map[string]any{"section_id": "two_step_verification", "fields": []map[string]any{
			{"field_id": "otp", "value": code},
			{"field_id": "trust_this_device", "value": trust},
		}},
	}
	response, raw, err = m.post(ctx, "/customers/login", payload, false)
	if err != nil {
		return Session{}, err
	}
	session, ok := sessionFromResponse(response, m.jar, m.base)
	if !ok {
		return Session{}, loginResponseError(raw)
	}
	if err := m.store.Save(session); err != nil {
		return Session{}, fmt.Errorf("save Takealot session: %w", err)
	}
	return session, nil
}

func (m *Manager) Refresh(ctx context.Context) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshLocked(ctx)
}

func (m *Manager) refreshLocked(ctx context.Context) (Session, error) {
	old, err := m.store.Load()
	if err != nil {
		return Session{}, err
	}
	m.restoreCookies(old)
	payload := map[string]any{"platform": "android", "refresh_token": old.RefreshToken, "tracking_id": old.TrackingID}
	response, _, err := m.post(ctx, "/customers/auth/refresh", payload, true)
	if err != nil {
		return Session{}, err
	}
	session, ok := sessionFromResponse(response, m.jar, m.base)
	if !ok {
		return Session{}, errors.New("Takealot refresh response did not contain a complete session")
	}
	session.CustomerID = firstNonEmpty(session.CustomerID, old.CustomerID)
	session.TrackingID = firstNonEmpty(session.TrackingID, old.TrackingID)
	session.Cookies = mergeCookies(old.Cookies, session.Cookies)
	if err := m.store.Save(session); err != nil {
		return Session{}, fmt.Errorf("save refreshed Takealot session: %w", err)
	}
	return session, nil
}

func (m *Manager) Status() (Session, error) { return m.store.Load() }
func (m *Manager) Logout() error            { return m.store.Delete() }

// DoJSON performs an authenticated mobile API request. It refreshes once after a 401.
func (m *Manager) DoJSON(ctx context.Context, method, path string, body any, destination any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.doJSONLocked(ctx, method, path, body, destination, true)
}

func (m *Manager) doJSONLocked(ctx context.Context, method, path string, body any, destination any, refreshOnUnauthorized bool) error {
	session, err := m.store.Load()
	if err != nil {
		return err
	}
	m.restoreCookies(session)
	status, responseBody, err := m.request(ctx, method, path, body, session)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized && refreshOnUnauthorized {
		if _, refreshErr := m.refreshLocked(ctx); refreshErr != nil {
			return fmt.Errorf("Takealot session expired and refresh failed: %w", refreshErr)
		}
		fresh, loadErr := m.store.Load()
		if loadErr != nil {
			return loadErr
		}
		status, responseBody, err = m.request(ctx, method, path, body, fresh)
		if err != nil {
			return err
		}
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return mobileAPIError(status, responseBody, "authenticated_request")
	}
	if destination == nil || len(responseBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, destination); err != nil {
		return fmt.Errorf("decode Takealot response: malformed JSON: %w", err)
	}
	return nil
}

func (m *Manager) post(ctx context.Context, path string, payload any, authenticated bool) (map[string]any, []byte, error) {
	var session Session
	if authenticated {
		var err error
		session, err = m.store.Load()
		if err != nil {
			return nil, nil, err
		}
		m.restoreCookies(session)
	}
	status, body, err := m.request(ctx, http.MethodPost, path, payload, session)
	if err != nil {
		return nil, body, err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, body, mobileAPIError(status, body, "request")
	}
	if len(body) == 0 {
		return map[string]any{}, body, nil
	}
	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, body, fmt.Errorf("decode Takealot response: malformed JSON: %w", err)
	}
	return response, body, nil
}

func (m *Manager) request(ctx context.Context, method, path string, body any, session Session) (int, []byte, error) {
	u, err := url.Parse(m.base + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return 0, nil, fmt.Errorf("build Takealot request URL: %w", err)
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("encode Takealot request: %w", err)
		}
		reader = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return 0, nil, fmt.Errorf("build Takealot request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", MobileUserAgent)
	req.Header.Set("X-Tal-Platform", "android")
	if session.JWT != "" {
		req.Header.Set("Authorization", "Bearer "+session.JWT)
	}
	if session.CSRFToken != "" {
		req.Header.Set("X-CSRF-Token", session.CSRFToken)
	}
	response, err := m.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("Takealot request failed: %w", err)
	}
	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(io.LimitReader(response.Body, 16<<20+1))
	if err != nil {
		return response.StatusCode, nil, fmt.Errorf("read Takealot response: %w", err)
	}
	if len(bodyBytes) > 16<<20 {
		return response.StatusCode, nil, errors.New("Takealot response exceeded 16 MiB limit")
	}
	return response.StatusCode, bodyBytes, nil
}

func (m *Manager) restoreCookies(session Session) {
	if m.jar == nil {
		return
	}
	u, err := url.Parse(m.base)
	if err != nil {
		return
	}
	cookies := make([]*http.Cookie, 0, len(session.Cookies)+4)
	for _, cookie := range session.Cookies {
		cookies = append(cookies, &http.Cookie{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path, Expires: cookie.Expires, Secure: cookie.Secure, HttpOnly: cookie.HttpOnly})
	}
	cookies = append(cookies, authCookies(session)...)
	m.jar.SetCookies(u, cookies)
}

func sessionFromResponse(response map[string]any, jar http.CookieJar, base string) (Session, bool) {
	info := asMap(response["auth_info"])
	if info == nil {
		info = asMap(response["data"])
	}
	if info == nil {
		return Session{}, false
	}
	session := Session{
		JWT:                   stringValue(info["jwt"]),
		IDToken:               stringValue(info["id_token"]),
		RefreshToken:          stringValue(info["refresh_token"]),
		CSRFToken:             stringValue(info["csrf_token"]),
		TrackingID:            stringValue(info["tracking_id"]),
		CustomerID:            stringValue(info["customer_id"]),
		AccessKey:             stringValue(info["access_key"]),
		PrivateKey:            stringValue(info["private_key"]),
		DID:                   stringValue(info["did"]),
		JWTExpiresAt:          parseExpiry(info["jwt_expires"], info["jwt_expires_at"]),
		IDTokenExpiresAt:      parseExpiry(info["id_token_expires"], info["id_token_expires_at"]),
		RefreshTokenExpiresAt: parseExpiry(info["refresh_token_expires"], info["refresh_token_expires_at"]),
	}
	if jar != nil {
		if u, err := url.Parse(base); err == nil {
			for _, cookie := range jar.Cookies(u) {
				session.Cookies = append(session.Cookies, Cookie{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path, Expires: cookie.Expires, Secure: cookie.Secure, HttpOnly: cookie.HttpOnly})
			}
		}
	}
	session.Cookies = append(session.Cookies, cookieValues(authCookies(session))...)
	// Refresh responses commonly omit customer_id; the caller preserves it from
	// the prior session before saving. Login still fails at Store.Save if it is
	// missing there.
	return session, session.JWT != "" && session.RefreshToken != ""
}

// The Android client sends these auth_info values as cookies as well as using
// the bearer header. Keeping both forms is required by some customer routes.
func authCookies(session Session) []*http.Cookie {
	result := make([]*http.Cookie, 0, 4)
	for name, value := range map[string]string{"taid": session.IDToken, "tal_jwt": session.JWT, "tal_csrf": session.CSRFToken, "did": session.DID} {
		if value != "" {
			result = append(result, &http.Cookie{Name: name, Value: value, Path: "/"})
		}
	}
	return result
}

func cookieValues(cookies []*http.Cookie) []Cookie {
	result := make([]Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		result = append(result, Cookie{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path, Expires: cookie.Expires, Secure: cookie.Secure, HttpOnly: cookie.HttpOnly})
	}
	return result
}

func mergeCookies(old, fresh []Cookie) []Cookie {
	if len(fresh) == 0 {
		return old
	}
	return fresh
}

func hasTwoStepChallenge(response map[string]any) bool {
	value := strings.ToLower(stringValue(response["two_step_verification"]))
	return value != "" && value != "disabled" && value != "false"
}

func loginResponseError(body []byte) error {
	if len(body) == 0 {
		return errors.New("Takealot login failed: response did not contain authentication data")
	}
	return errors.New("Takealot login failed: credentials were rejected or the response was incomplete")
}

func mobileAPIError(status int, body []byte, defaultCode string) error {
	code := defaultCode
	message := http.StatusText(status)
	lower := strings.ToLower(string(body))
	if strings.Contains(lower, "just a moment") || strings.Contains(lower, "challenge-platform") || strings.Contains(lower, "performing security verification") {
		code = "cloudflare_challenge"
		message = "Takealot returned a Cloudflare challenge; automated access is temporarily unavailable"
	} else {
		switch status {
		case http.StatusForbidden:
			code = "forbidden"
		case http.StatusNotFound:
			code = "not_found"
		case http.StatusTooManyRequests:
			code = "rate_limited"
		case http.StatusUnauthorized:
			code = "unauthorized"
		}
	}
	return &APIError{StatusCode: status, Code: code, Message: message}
}

func parseExpiry(values ...any) time.Time {
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			if parsed, err := time.Parse(time.RFC3339, typed); err == nil {
				return parsed
			}
			if number, err := strconv.ParseInt(typed, 10, 64); err == nil {
				return expiryFromUnix(number)
			}
		case float64:
			return expiryFromUnix(int64(typed))
		case json.Number:
			if number, err := typed.Int64(); err == nil {
				return expiryFromUnix(number)
			}
		}
	}
	return time.Time{}
}

func expiryFromUnix(number int64) time.Time {
	if number > 1e12 {
		return time.UnixMilli(number).UTC()
	}
	return time.Unix(number, 0).UTC()
}

func asMap(value any) map[string]any {
	valueMap, _ := value.(map[string]any)
	return valueMap
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
