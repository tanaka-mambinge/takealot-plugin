package config

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	HTTPTimeoutEnv     = "TAKEALOT_HTTP_TIMEOUT_SECONDS"
	DefaultHTTPTimeout = 30 * time.Second
	MaxHTTPTimeout     = 90 * time.Second
)

// HTTPTimeout reads the optional operation timeout override. Keeping the
// upper bound here prevents an environment setting from disabling timeouts.
func HTTPTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(HTTPTimeoutEnv))
	if raw == "" {
		return DefaultHTTPTimeout, nil
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 1 || time.Duration(seconds)*time.Second > MaxHTTPTimeout {
		return 0, fmt.Errorf("%s must be an integer between 1 and 90", HTTPTimeoutEnv)
	}
	return time.Duration(seconds) * time.Second, nil
}

// NewHTTPClient creates the bounded client used by production constructors.
// Execute validates the environment before running a command; the fallback
// here keeps package-level constructors safe when used directly by tests or
// embedding code.
func NewHTTPClient() *http.Client {
	timeout, err := HTTPTimeout()
	if err != nil {
		timeout = DefaultHTTPTimeout
	}
	return &http.Client{Timeout: timeout}
}
