package cli

import (
	"context"
	"crypto/rand"
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

type loginPageData struct {
	Token       string
	Path        string
	Email       string
	OTPRequired bool
	Message     string
	Error       bool
	Success     bool
	CustomerID  string
}

var loginPageTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Takealot CLI login</title>
<style>body{font:16px system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem;color:#182338}label{display:block;margin:1rem 0 .35rem}input{box-sizing:border-box;width:100%;padding:.7rem;border:1px solid #abb4c4;border-radius:.4rem;font-size:1rem}button{margin-top:1.2rem;padding:.7rem 1rem;background:#1079bf;color:#fff;border:0;border-radius:.4rem;font-size:1rem}.error{color:#a12626}.ok{color:#176b38}</style></head>
<body><h1>Sign in to Takealot</h1><p>Credentials are sent only to this local CLI page and then directly to Takealot's mobile API. Tokens are saved in your OS keyring and are not shown here.</p>
{{if .Message}}<p class="{{if .Error}}error{{else}}ok{{end}}">{{.Message}}</p>{{end}}
{{if not .Success}}<form method="post" action="{{.Path}}?token={{.Token}}">
<label for="email">Email</label><input id="email" name="email" type="email" value="{{.Email}}" autocomplete="username" required>
<label for="password">Password</label><input id="password" name="password" type="password" autocomplete="current-password" {{if not .OTPRequired}}required{{end}}>
{{if .OTPRequired}}<p>Takealot requires a one-time password. Your password is retained only in this running CLI process for this login attempt; leave the password field blank to reuse it.</p>{{end}}
<label for="otp">One-time password (only if Takealot asks for it)</label><input id="otp" name="otp" inputmode="numeric" autocomplete="one-time-code">
<label><input name="trust_device" type="checkbox" value="true" checked> Trust this device</label>
<button type="submit">Sign in</button></form>{{else}}<p>You can close this tab and return to the CLI.</p>{{end}}
</body></html>`))

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
	fmt.Fprintf(command.ErrOrStderr(), "Open this local Takealot login page: %s\n", loginURL)
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
