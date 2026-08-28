package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteLoginPageEmbedsWordmarkAndEscapesStatus(t *testing.T) {
	response := httptest.NewRecorder()

	writeLoginPage(response, http.StatusBadRequest, loginPageData{
		Email:   "shopper@example.com",
		Message: `<script>alert("no")</script>`,
		Error:   true,
	})

	body := response.Body.String()
	for _, expected := range []string{
		`<img src="data:image/svg`,
		`base64,`,
		`Connect your Takealot account`,
		`Takealot shopping plugin`,
		`autocomplete="username"`,
		`id="toggle-password"`,
		`aria-label="Show password"`,
		`aria-controls="password"`,
		`input[type=password],input#password`,
		`button.password-toggle{position:absolute`,
		`height:46px`,
		`role="alert"`,
		`&lt;script&gt;alert(&#34;no&#34;)&lt;/script&gt;`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("login page does not contain %q", expected)
		}
	}
	if strings.Contains(body, `<script>alert("no")</script>`) {
		t.Fatal("login page rendered an unescaped status message")
	}
	if strings.Contains(body, "Local and private") || strings.Contains(body, "secure sign-in") {
		t.Fatal("login page still renders removed reassurance copy")
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	if got := response.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
}

func TestWriteLoginPageSuccessState(t *testing.T) {
	response := httptest.NewRecorder()

	writeLoginPage(response, http.StatusOK, loginPageData{Success: true})

	body := response.Body.String()
	for _, expected := range []string{
		`You’re signed in`,
		`You can close this tab and return to your shopping assistant.`,
		`<img src="data:image/svg`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("success page does not contain %q", expected)
		}
	}
	if strings.Contains(body, `<form`) {
		t.Fatal("success page still contains the login form")
	}
}
