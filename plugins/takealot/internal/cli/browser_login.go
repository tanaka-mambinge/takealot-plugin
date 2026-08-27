package cli

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/t12e/takealot-cli/internal/client"
)

const loginPagePath = "/takealot-login"

// takealotWordmark is embedded so the local login page remains self-contained.
// The asset is the Takealot wordmark, not the square favicon/icon.
//
//go:embed assets/takealot-wordmark.svg
var takealotWordmark []byte

type loginPageData struct {
	Token       string
	Path        string
	Email       string
	OTPRequired bool
	Message     string
	Error       bool
	Success     bool
	CustomerID  string
	LogoDataURI template.URL
}

var loginPageTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#1079bf"><title>Sign in to Takealot</title>
<style>
:root{color-scheme:light;--blue:#1079bf;--blue-dark:#075a91;--ink:#182338;--muted:#5d6b7c;--line:#d7e1ea;--canvas:#f3f8fb;--surface:#fff;--danger:#a12626;--success:#176b38;--radius:18px}
*{box-sizing:border-box}
html,body{min-height:100%}
body{margin:0;background:var(--canvas);color:var(--ink);font:16px/1.55 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
.page{min-height:100dvh;display:grid;place-items:center;padding:24px 16px}
.login-shell{width:min(100%,430px)}
.brand-banner{display:flex;align-items:center;justify-content:center;min-height:78px;padding:16px 22px;background:var(--blue);border-radius:var(--radius) var(--radius) 0 0;color:#fff}
.brand-lockup{display:flex;align-items:center;min-width:0;padding:9px 13px;background:#fff;border-radius:10px}
.brand-lockup img{display:block;width:min(224px,100%);height:auto}
.panel{padding:30px;background:var(--surface);border:1px solid rgba(24,35,56,.1);border-top:0;border-radius:0 0 var(--radius) var(--radius);box-shadow:0 18px 42px rgba(24,35,56,.12)}
h1{margin:0;font-size:28px;line-height:1.15;letter-spacing:-.025em;font-weight:650}
.intro{margin:10px 0 24px;color:var(--muted);font-size:16px;line-height:1.55}
.field{margin-top:17px}
label{display:block;margin-bottom:7px;font-size:14px;font-weight:650}
input[type=email],input[type=password],input[name=otp]{display:block;width:100%;min-height:48px;padding:11px 13px;border:1px solid #b8c5d2;border-radius:10px;background:#fff;color:var(--ink);font:inherit;font-size:16px;transition:border-color .15s ease,box-shadow .15s ease}
input[type=email]:focus,input[type=password]:focus,input[name=otp]:focus{border-color:var(--blue);outline:2px solid var(--blue);outline-offset:-1px}
.hint{margin:8px 0 0;color:var(--muted);font-size:14px;line-height:1.45}
.otp-note{margin:16px 0 0;padding:12px 14px;border-left:3px solid var(--blue);background:#eef7fc;color:#38536b;font-size:14px;line-height:1.5}
.checkbox-row{display:flex;align-items:flex-start;gap:10px;margin-top:18px;font-weight:400;font-size:14px;color:var(--muted)}
.checkbox-row input{width:18px;height:18px;flex:0 0 auto;margin:1px 0 0;accent-color:var(--blue)}
.checkbox-row label{margin:0;font-weight:400}
.status{margin:20px 0 0;padding:12px 14px;border-radius:10px;font-size:14px;line-height:1.45}
.status.error{background:#fff1f0;color:var(--danger)}
.status.ok{background:#effaf3;color:var(--success)}
button{display:block;width:100%;min-height:50px;margin-top:24px;padding:12px 16px;border:0;border-radius:10px;background:var(--blue);color:#fff;font:inherit;font-weight:650;cursor:pointer;transition:background .15s ease,transform .05s ease}
button:hover{background:var(--blue-dark)}
button:active{transform:translateY(1px)}
button:focus-visible{outline:2px solid var(--blue-dark);outline-offset:3px}
button:disabled{background:#8bb9d5;cursor:wait}
.security-note{margin:24px 0 0;padding-top:18px;border-top:1px solid rgba(24,35,56,.1);color:var(--muted);font-size:13px;line-height:1.5}
.security-note strong{display:block;margin-bottom:3px;color:var(--ink);font-weight:650}
.success-panel{text-align:center}
.success-mark{display:grid;place-items:center;width:56px;height:56px;margin:0 auto 18px;border-radius:50%;background:#effaf3;color:var(--success);font-size:28px;font-weight:700}
.close-note{margin:12px 0 0;color:var(--muted)}
@media (max-width:420px){.page{padding:12px 10px}.brand-banner{padding:14px 16px}.brand-lockup img{width:min(224px,100%)}.panel{padding:24px 18px}h1{font-size:26px}}
@media (prefers-reduced-motion:reduce){*{scroll-behavior:auto!important;transition:none!important}}
</style></head>
<body><main class="page"><section class="login-shell" aria-labelledby="page-title">
<header class="brand-banner"><div class="brand-lockup"><img src="{{.LogoDataURI}}" width="224" height="46" alt="Takealot"></div></header>
<div class="panel{{if .Success}} success-panel{{end}}">
{{if not .Success}}<h1 id="page-title">Connect your Takealot account</h1><p class="intro">Sign in once to let the Takealot shopping plugin research products and manage your wishlist.</p>
{{if .Message}}<p class="status {{if .Error}}error{{else}}ok{{end}}" role="{{if .Error}}alert{{else}}status{{end}}" aria-live="polite">{{.Message}}</p>{{end}}
<form method="post" action="{{.Path}}?token={{.Token}}" onsubmit="this.querySelector('button').disabled=true;this.querySelector('button').setAttribute('aria-busy','true');this.querySelector('button').textContent='Signing in…';">
<div class="field"><label for="email">Email address</label><input id="email" name="email" type="email" value="{{.Email}}" autocomplete="username" autocapitalize="none" spellcheck="false" required autofocus></div>
<div class="field"><label for="password">Password</label><input id="password" name="password" type="password" autocomplete="current-password" {{if not .OTPRequired}}required{{end}}></div>
{{if .OTPRequired}}<p class="otp-note">Takealot requested a one-time password. Your password is retained only in this running CLI process for this login attempt, so you can leave the password field blank when submitting the code.</p>{{end}}
<div class="field"><label for="otp">One-time password <span class="hint">(only if requested)</span></label><input id="otp" name="otp" inputmode="numeric" autocomplete="one-time-code"></div>
<div class="checkbox-row"><input id="trust-device" name="trust_device" type="checkbox" value="true" checked><label for="trust-device">Trust this device for future shopping requests</label></div>
<button type="submit">{{if .OTPRequired}}Verify and finish{{else}}Sign in securely{{end}}</button></form>
<p class="security-note"><strong>Local and private</strong>This page stays on your computer. Your password goes directly to Takealot, and the plugin stores only a session token in your OS keyring.</p>
{{else}}<div class="success-mark" aria-hidden="true">✓</div><h1 id="page-title">You’re signed in</h1><p class="intro">Your Takealot session is saved securely for the plugin.</p><p class="close-note">You can close this tab and return to your shopping assistant.</p>{{end}}
</div></section></main></body></html>`))

func readPasswordStdin(command *cobra.Command) (string, string, error) {
	data, err := io.ReadAll(command.InOrStdin())
	if err != nil {
		return "", "", fmt.Errorf("read password from stdin: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return "", "", errors.New("password-stdin did not provide a password")
	}
	otp := ""
	if len(lines) > 1 {
		otp = strings.TrimSpace(lines[1])
	}
	return lines[0], otp, nil
}

func runBrowserLogin(command *cobra.Command, initialEmail string, trustDevice bool) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start local login page: %w", err)
	}
	defer listener.Close()
	token, err := randomToken()
	if err != nil {
		return fmt.Errorf("create local login token: %w", err)
	}
	api := client.NewAuthenticated()
	resultCh := make(chan error, 1)
	var once sync.Once
	finish := func(result error) { once.Do(func() { resultCh <- result }) }
	var pendingMu sync.Mutex
	var pendingEmail, pendingPassword string
	var pendingUntil time.Time

	mux := http.NewServeMux()
	mux.HandleFunc(loginPagePath, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("token") != token {
			writeLoginPage(writer, http.StatusNotFound, loginPageData{Message: "Login page not found.", Error: true})
			return
		}
		if request.Method == http.MethodGet {
			writeLoginPage(writer, http.StatusOK, loginPageData{Token: token, Email: initialEmail, Path: loginPagePath})
			return
		}
		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := request.ParseForm(); err != nil {
			writeLoginPage(writer, http.StatusBadRequest, loginPageData{Token: token, Path: loginPagePath, Message: "The submitted form was invalid.", Error: true})
			return
		}
		email := strings.TrimSpace(request.FormValue("email"))
		password := request.FormValue("password")
		otp := strings.TrimSpace(request.FormValue("otp"))
		pendingMu.Lock()
		if password == "" && email != "" && email == pendingEmail && time.Now().Before(pendingUntil) {
			password = pendingPassword
		}
		pendingMu.Unlock()
		if email == "" || password == "" {
			writeLoginPage(writer, http.StatusBadRequest, loginPageData{Token: token, Path: loginPagePath, Email: email, Message: "Email and password are required.", Error: true})
			return
		}
		_, trustSubmitted := request.Form["trust_device"]
		trust := request.FormValue("trust_device") == "true"
		if !trustSubmitted {
			trust = trustDevice
		}
		status, loginErr := api.Login(request.Context(), email, password, func(context.Context) (string, bool, error) {
			if otp == "" {
				return "", false, errors.New("Takealot requested an OTP; enter it and submit the local page again")
			}
			return otp, trust, nil
		})
		if loginErr != nil {
			otpRequired := otp == "" && strings.Contains(loginErr.Error(), "requested an OTP")
			if otpRequired {
				pendingMu.Lock()
				pendingEmail, pendingPassword, pendingUntil = email, password, time.Now().Add(5*time.Minute)
				pendingMu.Unlock()
			}
			writeLoginPage(writer, http.StatusBadRequest, loginPageData{Token: token, Path: loginPagePath, Email: email, OTPRequired: otpRequired, Message: loginErr.Error(), Error: true})
			return
		}
		pendingMu.Lock()
		pendingEmail, pendingPassword = "", ""
		pendingUntil = time.Time{}
		pendingMu.Unlock()
		writeLoginPage(writer, http.StatusOK, loginPageData{Token: token, Path: loginPagePath, Success: true, CustomerID: status.CustomerID, Message: "Signed in successfully. Your session is saved in the OS keyring."})
		finish(nil)
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			finish(fmt.Errorf("local login page stopped: %w", serveErr))
		}
	}()
	loginURL := "http://" + listener.Addr().String() + loginPagePath + "?" + url.Values{"token": []string{token}}.Encode()
	fmt.Fprintf(command.OutOrStdout(), "Open this local Takealot login page: %s\n", loginURL)
	_ = openBrowser(loginURL)

	select {
	case err := <-resultCh:
		shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
		if err != nil {
			return err
		}
		if options.json {
			return writeJSON(command.OutOrStdout(), map[string]any{"authenticated": true, "message": "Takealot session saved in the OS keyring"})
		}
		fmt.Fprintln(command.OutOrStdout(), "Takealot login completed; session saved in the OS keyring.")
		return nil
	case <-command.Context().Done():
		_ = server.Shutdown(context.Background())
		return command.Context().Err()
	}
}

func writeLoginPage(writer http.ResponseWriter, status int, data loginPageData) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	data.Path = loginPagePath
	data.LogoDataURI = template.URL("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(takealotWordmark))
	_ = loginPageTemplate.Execute(writer, data)
}

func randomToken() (string, error) {
	data := make([]byte, 24)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func openBrowser(target string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	return command.Start()
}
