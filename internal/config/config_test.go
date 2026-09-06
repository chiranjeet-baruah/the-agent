package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirWithConfig writes yaml into a fresh temp dir's config/config.yaml and
// chdirs the test there, so config.Load's relative "config" search path
// resolves to this fixture rather than the real repo config. t.Chdir forbids
// t.Parallel, so these tests must stay sequential.
func chdirWithConfig(t *testing.T, yaml string) {
	t.Helper()
	dir := t.TempDir()
	if yaml != "" {
		if err := os.Mkdir(filepath.Join(dir, "config"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config", "config.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}

// clearEnv clears every env var config.Load's AutomaticEnv could pick up, so
// a real key set on the developer's machine can't leak into a test case.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"PROVIDER", "MODEL", "PORT", "OPENAI_API_KEY", "GROQ_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(k, "")
	}
}

const twoProviderYAML = `
providers:
  groq:
    model: [llama-3.1-8b-instant]
    base_url: https://api.groq.com/openai/v1
    api_key_env: GROQ_API_KEY
  openrouter:
    model: [some/model:free]
    base_url: https://openrouter.ai/api/v1
    api_key_env: OPENROUTER_API_KEY
`

func TestLoad(t *testing.T) {
	tests := []struct {
		name             string
		yaml             string
		env              map[string]string
		wantErrSubstring string
		wantPort         string
		wantProviders    []string // Candidate.Provider, in returned order
		wantOverride     string   // ModelOverride of the first (only) candidate
	}{
		{
			name:          "one key set selects that provider",
			yaml:          twoProviderYAML,
			env:           map[string]string{"GROQ_API_KEY": "gsk_x"},
			wantPort:      "8080",
			wantProviders: []string{"groq"},
		},
		{
			name:          "both keys set yields alphabetical order",
			yaml:          twoProviderYAML,
			env:           map[string]string{"GROQ_API_KEY": "gsk_x", "OPENROUTER_API_KEY": "sk-or-x"},
			wantPort:      "8080",
			wantProviders: []string{"groq", "openrouter"},
		},
		{
			name:             "no key set is an error naming the checked env vars",
			yaml:             twoProviderYAML,
			env:              nil,
			wantErrSubstring: "GROQ_API_KEY",
		},
		{
			name:          "explicit provider with its key set",
			yaml:          twoProviderYAML,
			env:           map[string]string{"PROVIDER": "openrouter", "OPENROUTER_API_KEY": "sk-or-x"},
			wantPort:      "8080",
			wantProviders: []string{"openrouter"},
		},
		{
			name:          "explicit provider plus MODEL becomes ModelOverride",
			yaml:          twoProviderYAML,
			env:           map[string]string{"PROVIDER": "groq", "GROQ_API_KEY": "gsk_x", "MODEL": "llama-guard-3-8b"},
			wantPort:      "8080",
			wantProviders: []string{"groq"},
			wantOverride:  "llama-guard-3-8b",
		},
		{
			name:          "MODEL without PROVIDER is ignored, not hoisted into auto-select",
			yaml:          twoProviderYAML,
			env:           map[string]string{"GROQ_API_KEY": "gsk_x", "MODEL": "should-be-ignored"},
			wantPort:      "8080",
			wantProviders: []string{"groq"},
			wantOverride:  "",
		},
		{
			name:             "explicit unknown provider fails fast",
			yaml:             twoProviderYAML,
			env:              map[string]string{"PROVIDER": "made-up"},
			wantErrSubstring: `unknown provider "made-up"`,
		},
		{
			name:             "explicit provider with key unset fails fast",
			yaml:             twoProviderYAML,
			env:              map[string]string{"PROVIDER": "groq"},
			wantErrSubstring: "GROQ_API_KEY is required",
		},
		{
			name:             "missing providers map fails fast",
			yaml:             "port: \"9090\"\n",
			wantErrSubstring: "no providers configured",
		},
		{
			name:             "empty model list on an entry fails fast",
			yaml:             "providers:\n  groq:\n    model: []\n    base_url: https://api.groq.com/openai/v1\n    api_key_env: GROQ_API_KEY\n",
			wantErrSubstring: "providers.groq.model must list at least one model",
		},
		{
			name:          "port falls back to the 8080 default",
			yaml:          twoProviderYAML,
			env:           map[string]string{"GROQ_API_KEY": "gsk_x"},
			wantPort:      "8080",
			wantProviders: []string{"groq"},
		},
		{
			name:          "PORT override is honored",
			yaml:          twoProviderYAML,
			env:           map[string]string{"GROQ_API_KEY": "gsk_x", "PORT": "9090"},
			wantPort:      "9090",
			wantProviders: []string{"groq"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			chdirWithConfig(t, tt.yaml)

			candidates, port, err := Load()

			if tt.wantErrSubstring != "" {
				if err == nil {
					t.Fatalf("Load() = %v, %v, nil error; want error containing %q", candidates, port, tt.wantErrSubstring)
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstring) {
					t.Fatalf("Load() error = %q; want it to contain %q", err.Error(), tt.wantErrSubstring)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if port != tt.wantPort {
				t.Errorf("port = %q, want %q", port, tt.wantPort)
			}
			gotProviders := make([]string, len(candidates))
			for i, c := range candidates {
				gotProviders[i] = c.Provider
			}
			if !slicesEqual(gotProviders, tt.wantProviders) {
				t.Errorf("providers = %v, want %v", gotProviders, tt.wantProviders)
			}
			if len(candidates) > 0 && candidates[0].ModelOverride != tt.wantOverride {
				t.Errorf("candidates[0].ModelOverride = %q, want %q", candidates[0].ModelOverride, tt.wantOverride)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
