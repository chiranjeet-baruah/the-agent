package openaicompat_outbound_adapter

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/adk/v2/model"
)

// TestGenerateContentGuards checks the two fail-fast guards that reject a
// request before ever reaching the network: streaming and tool calls are
// both unsupported by this hand-rolled Chat Completions model (see llm.go).
func TestGenerateContentGuards(t *testing.T) {
	tests := []struct {
		name             string
		req              *model.LLMRequest
		stream           bool
		wantErrSubstring string
	}{
		{
			name:             "streaming requested",
			req:              &model.LLMRequest{},
			stream:           true,
			wantErrSubstring: "streaming not supported",
		},
		{
			name:             "tools requested",
			req:              &model.LLMRequest{Tools: map[string]any{"search": struct{}{}}},
			stream:           false,
			wantErrSubstring: "tool calling not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newLLMModel("unused-model", "http://127.0.0.1:0", "unused-key")

			var gotErr error
			called := false
			for _, err := range m.GenerateContent(context.Background(), tt.req, tt.stream) {
				called = true
				gotErr = err
			}

			if !called {
				t.Fatal("GenerateContent yielded nothing; want exactly one error result")
			}
			if gotErr == nil {
				t.Fatalf("GenerateContent() error = nil, want error containing %q", tt.wantErrSubstring)
			}
			if !strings.Contains(gotErr.Error(), tt.wantErrSubstring) {
				t.Errorf("GenerateContent() error = %q, want it to contain %q", gotErr.Error(), tt.wantErrSubstring)
			}
		})
	}
}
