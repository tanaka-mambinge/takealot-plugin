package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type memoryBackend struct{ value string }

func (b *memoryBackend) Get(string, string) (string, error) {
	if b.value == "" {
		return "", errors.New("secret not found in keyring")
	}
	return b.value, nil
}
func (b *memoryBackend) Set(_, _, value string) error { b.value = value; return nil }
func (b *memoryBackend) Delete(_, _ string) error     { b.value = ""; return nil }

func TestLoginMatchesMobileFlowAndStoresSession(t *testing.T) {
	backend := &memoryBackend{}
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodPost || request.URL.Path != "/customers/login" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("User-Agent") != MobileUserAgent || request.Header.Get("X-Tal-Platform") != "android" {
			t.Fatalf("mobile headers missing: %#v", request.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		sections := body["sections"].([]any)
		if len(sections) != 1 {
			t.Fatalf("unexpected initial sections: %#v", sections)
		}
		writer.Header().Add("Set-Cookie", "__cf_bm=challenge-cookie; Path=/")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"auth_info":{"jwt":"jwt-secret","refresh_token":"refresh-secret","customer_id":123,"csrf_token":"csrf","tracking_id":"track","jwt_expires_at":"2026-08-27T12:00:00Z"}}`))
	}))
	defer server.Close()

	manager := NewManager(server.Client(), server.URL, NewStoreWithBackend(backend))
	session, err := manager.Login(context.Background(), "person@example.com", "password-not-logged", nil)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || session.CustomerID != "123" || session.JWT != "jwt-secret" || !hasCookie(session.Cookies, "__cf_bm", "challenge-cookie") || !hasCookie(session.Cookies, "tal_jwt", "jwt-secret") {
		t.Fatalf("session was not normalized/stored correctly: %#v", session)
	}
	stored, err := manager.Status()
	if err != nil || stored.RefreshToken != "refresh-secret" {
		t.Fatalf("stored session unavailable: %#v, %v", stored, err)
	}
}

func hasCookie(cookies []Cookie, name, value string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value == value {
			return true
		}
	}
	return false
}

func TestLoginTwoStepVerificationRetainsCloudflareCookie(t *testing.T) {
	backend := &memoryBackend{}
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if requests == 1 {
			writer.Header().Add("Set-Cookie", "__cf_bm=challenge-cookie; Path=/")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"two_step_verification":"enabled_untrusted","otp_status":{"status":"unverified"}}`))
			return
		}
		if !strings.Contains(request.Header.Get("Cookie"), "__cf_bm=challenge-cookie") {
			t.Errorf("second login did not retain Cloudflare cookie: %q", request.Header.Get("Cookie"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		sections := body["sections"].([]any)
		if len(sections) != 2 || sections[1].(map[string]any)["section_id"] != "two_step_verification" {
			t.Errorf("OTP section missing: %#v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"auth_info":{"jwt":"jwt","refresh_token":"refresh","customer_id":"customer"}}`))
	}))
	defer server.Close()

	manager := NewManager(server.Client(), server.URL, NewStoreWithBackend(backend))
	_, err := manager.Login(context.Background(), "person@example.com", "password", func(context.Context) (string, bool, error) { return "12345", true, nil })
	if err != nil || requests != 2 {
		t.Fatalf("two-step login failed: %v (requests=%d)", err, requests)
	}
}

func TestAuthenticatedRequestRefreshesOnce(t *testing.T) {
	backend := &memoryBackend{}
	initial := Session{JWT: "expired", RefreshToken: "refresh", CustomerID: "customer", TrackingID: "tracking"}
	if err := NewStoreWithBackend(backend).Save(initial); err != nil {
		t.Fatal(err)
	}
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if request.URL.Path == "/customers/auth/refresh" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"auth_info":{"jwt":"fresh","refresh_token":"rotated","customer_id":"customer","tracking_id":"tracking"}}`))
			return
		}
		if request.Header.Get("Authorization") == "Bearer expired" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.Header.Get("Authorization") != "Bearer fresh" {
			t.Errorf("unexpected authorization header: %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	manager := NewManager(server.Client(), server.URL, NewStoreWithBackend(backend))
	var result map[string]any
	if err := manager.DoJSON(context.Background(), http.MethodGet, "/customers/customer/summary", nil, &result); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || !result["ok"].(bool) {
		t.Fatalf("refresh/retry did not complete: calls=%d result=%#v", calls, result)
	}
	stored, _ := manager.Status()
	if stored.JWT != "fresh" || stored.RefreshToken != "rotated" {
		t.Fatalf("rotated credentials were not stored: %#v", stored)
	}
}

func TestLoginErrorsDoNotEchoSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte("password-secret and jwt-secret"))
	}))
	defer server.Close()
	manager := NewManager(server.Client(), server.URL, NewStoreWithBackend(&memoryBackend{}))
	_, err := manager.Login(context.Background(), "person@example.com", "password-secret", nil)
	if err == nil || strings.Contains(err.Error(), "password-secret") || strings.Contains(err.Error(), "jwt-secret") {
		t.Fatalf("login error leaked secret or was absent: %v", err)
	}
}
