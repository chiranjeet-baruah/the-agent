// Package openaicompat_outbound_adapter implements the outbound
// AgentEnginePort against any backend that exposes an OpenAI-compatible
// Chat Completions API, via a google/adk-go LlmAgent + Runner.
package openaicompat_outbound_adapter

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/genai"

	"github.com/chiranjeet14/the-agent/internal/port/outbound"
)

// Config holds the settings needed to build the adapter's model.
type Config struct {
	ModelName string
	BaseURL   string
}

// Adapter implements outbound.AgentEnginePort.
type Adapter struct {
	runner  *runner.Runner
	baseURL string
}

var _ outbound.AgentEnginePort = (*Adapter)(nil)

// New builds the LLM agent and returns an Adapter ready to process turns.
func New(cfg Config) (*Adapter, error) {
	llm := newLLMModel(cfg.ModelName, cfg.BaseURL)

	a, err := llmagent.New(llmagent.Config{
		Name:        "the_agent",
		Model:       llm,
		Description: "A minimal learning agent.",
		Instruction: "You are a helpful assistant.",
		// Tools omitted: llmModel has no tool-call conversion and errors
		// if Tools is set (see llm.go).
	})
	if err != nil {
		return nil, fmt.Errorf("openaicompat_outbound_adapter: init llmagent: %w", err)
	}

	r, err := runner.NewInMemory("the-agent", a)
	if err != nil {
		return nil, fmt.Errorf("openaicompat_outbound_adapter: init runner: %w", err)
	}
	return &Adapter{runner: r, baseURL: cfg.BaseURL}, nil
}

// RunTurn runs one turn of the agent and returns its reply text.
func (a *Adapter) RunTurn(ctx context.Context, userID, sessionID, message string) (string, error) {
	msg := genai.NewContentFromText(message, genai.RoleUser)

	var reply strings.Builder
	for ev, err := range a.runner.Run(ctx, userID, sessionID, msg, adkagent.RunConfig{}) {
		if err != nil {
			return "", err
		}
		if !ev.IsFinalResponse() || ev.Content == nil {
			continue
		}
		for _, part := range ev.Content.Parts {
			if part.Text != "" {
				reply.WriteString(part.Text)
			}
		}
	}
	return reply.String(), nil
}

// Ping checks that the configured backend is reachable, for startup checks.
func (a *Adapter) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}
