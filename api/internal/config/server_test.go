package config

import (
	"os"
	"testing"
)

func TestLoadServerConfig(t *testing.T) {
	tests := []struct {
		name         string
		env          map[string]string
		expectError  bool
		expectedPort string
	}{
		{
			name:        "port missing",
			env:         map[string]string{},
			expectError: true,
		},
		{
			name: "port invalid",
			env: map[string]string{
				"PORT": "invalid",
			},
			expectError: true,
		},
		{
			name: "all set",
			env: map[string]string{
				"PORT": "8080",
			},
			expectError:  false,
			expectedPort: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalPort := os.Getenv("PORT")
			defer os.Setenv("PORT", originalPort)

			os.Unsetenv("PORT")
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			cfg, err := LoadServerConfig()
			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}

			if !tt.expectError {
				if err != nil {
					t.Fatalf("did not expect error but got: %v", err)
				}
				if cfg.Port != tt.expectedPort {
					t.Errorf("expected port %s, got %s", tt.expectedPort, cfg.Port)
				}
			}
		})
	}
}
