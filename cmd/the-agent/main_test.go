package main

import (
	"strings"
	"testing"

	"github.com/chiranjeet-baruah/the-agent/internal/config"
)

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name             string
		candidate        config.Candidate
		live             []string
		want             string
		wantErrSubstring string
	}{
		{
			name:      "override wins even when not in the live list",
			candidate: config.Candidate{ModelOverride: "not-live-at-all", ModelPreferences: []string{"llama-3.1-8b-instant"}},
			live:      []string{"llama-3.1-8b-instant"},
			want:      "not-live-at-all",
		},
		{
			name:      "first preference present in the live list wins, regardless of the live list's own order",
			candidate: config.Candidate{ModelPreferences: []string{"openai/gpt-oss-20b", "llama-3.1-8b-instant"}},
			live:      []string{"whisper-large-v3", "llama-3.1-8b-instant", "openai/gpt-oss-20b"},
			want:      "openai/gpt-oss-20b",
		},
		{
			name:      "second preference wins when the first isn't live",
			candidate: config.Candidate{ModelPreferences: []string{"retired-model", "llama-3.1-8b-instant"}},
			live:      []string{"llama-3.1-8b-instant"},
			want:      "llama-3.1-8b-instant",
		},
		{
			name:             "no preference live is an error naming both lists",
			candidate:        config.Candidate{ModelPreferences: []string{"retired-model"}},
			live:             []string{"whisper-large-v3"},
			wantErrSubstring: "retired-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveModel(tt.candidate, tt.live)

			if tt.wantErrSubstring != "" {
				if err == nil {
					t.Fatalf("resolveModel() = %q, nil error; want error containing %q", got, tt.wantErrSubstring)
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstring) {
					t.Fatalf("resolveModel() error = %q; want it to contain %q", err.Error(), tt.wantErrSubstring)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveModel() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveModel() = %q, want %q", got, tt.want)
			}
		})
	}
}
