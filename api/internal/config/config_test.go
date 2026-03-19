package config

import (
	"os"
	"testing"
)

func TestNotSetErr(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		expected string
	}{
		{
			name:     "simple var name",
			envVar:   "PORT",
			expected: `"PORT" environment variable not set`,
		},
		{
			name:     "compound var name",
			envVar:   "GITHUB_APP_ID",
			expected: `"GITHUB_APP_ID" environment variable not set`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := notSetErr(tt.envVar)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, err.Error())
			}
		})
	}
}

func TestAppConfigErrors(t *testing.T) {
	vars := []string{"GITHUB_APP_ID", "GITHUB_APP_WEBHOOK_SECRET", "GITHUB_APP_PRIVATE_KEY"}

	tests := []struct {
		name        string
		env         map[string]string
		expectedErr string
	}{
		{
			name:        "GITHUB_APP_ID not set",
			env:         map[string]string{},
			expectedErr: `"GITHUB_APP_ID" environment variable not set`,
		},
		{
			name: "GITHUB_APP_WEBHOOK_SECRET not set",
			env: map[string]string{
				"GITHUB_APP_ID": "123",
			},
			expectedErr: `"GITHUB_APP_WEBHOOK_SECRET" environment variable not set`,
		},
		{
			name: "GITHUB_APP_PRIVATE_KEY not set",
			env: map[string]string{
				"GITHUB_APP_ID":             "123",
				"GITHUB_APP_WEBHOOK_SECRET": "secret",
			},
			expectedErr: `"GITHUB_APP_PRIVATE_KEY" environment variable not set`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := make(map[string]string, len(vars))
			for _, k := range vars {
				orig[k] = os.Getenv(k)
				os.Unsetenv(k)
			}
			defer func() {
				for k, v := range orig {
					os.Setenv(k, v)
				}
			}()

			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			_, err := LoadAppConfig()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.expectedErr {
				t.Errorf("expected %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestServerConfigErrors(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		expectedErr string
	}{
		{
			name:        "PORT not set",
			env:         map[string]string{},
			expectedErr: `"PORT" environment variable not set`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := os.Getenv("PORT")
			os.Unsetenv("PORT")
			defer os.Setenv("PORT", orig)

			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			_, err := LoadServerConfig()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.expectedErr {
				t.Errorf("expected %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}
