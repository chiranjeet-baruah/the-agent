// Package agent wires up the ADK agent and model, isolated from HTTP concerns
// so that later increments (e.g. adding tools) only need to change this file.
package agent

import (
	"context"
	"fmt"

	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/adk/v2/runner"
)

// Config holds the settings needed to build the agent's model.
type Config struct {
	ModelName     string
	OllamaBaseURL string
	OllamaAPIKey  string
}

// New builds the LLM agent and returns a Runner ready to process turns.
func New(ctx context.Context, cfg Config) (*runner.Runner, error) {
	llm, err := openaimodel.NewModel(ctx, cfg.ModelName, &openaimodel.ClientConfig{
		APIKey:  cfg.OllamaAPIKey,
		BaseURL: cfg.OllamaBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: init model: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "the_agent",
		Model:       llm,
		Description: "A minimal learning agent.",
		Instruction: "You are a helpful assistant.",
		// Tools intentionally omitted — increment 2 adds entries here.
	})
	if err != nil {
		return nil, fmt.Errorf("agent: init llmagent: %w", err)
	}

	r, err := runner.NewInMemory("the-agent", a)
	if err != nil {
		return nil, fmt.Errorf("agent: init runner: %w", err)
	}
	return r, nil
}
