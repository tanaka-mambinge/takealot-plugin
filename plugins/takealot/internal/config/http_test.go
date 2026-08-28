package config

import (
	"testing"
	"time"
)

func TestHTTPTimeout(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv(HTTPTimeoutEnv, "")
		got, err := HTTPTimeout()
		if err != nil || got != DefaultHTTPTimeout {
			t.Fatalf("HTTPTimeout() = %s, %v; want %s", got, err, DefaultHTTPTimeout)
		}
	})

	t.Run("override", func(t *testing.T) {
		t.Setenv(HTTPTimeoutEnv, "90")
		got, err := HTTPTimeout()
		if err != nil || got != 90*time.Second {
			t.Fatalf("HTTPTimeout() = %s, %v; want 90s", got, err)
		}
	})

	for _, value := range []string{"0", "91", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(HTTPTimeoutEnv, value)
			if _, err := HTTPTimeout(); err == nil {
				t.Fatalf("HTTPTimeout() accepted invalid value %q", value)
			}
		})
	}
}
