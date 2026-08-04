// Package agent wires up the ADK agent and model, isolated from HTTP concerns
// so that later increments (e.g. adding tools) only need to change this file.
package agent

import (
	"context"
	"fmt"

	"github.com/chiranjeet14/the-agent/internal/dmrmodel"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
)

// Config holds the settings needed to build the agent's model.
type Config struct {
	ModelName string
	BaseURL   string
}

// New builds the LLM agent and returns a Runner ready to process turns.
// ctx is unused for now (dmrmodel.New doesn't need it) but kept for parity
// with other model constructors' signatures (e.g. adk-go's gemini.NewModel).
func New(ctx context.Context, cfg Config) (*runner.Runner, error) {
	llm := dmrmodel.New(cfg.ModelName, cfg.BaseURL)

	a, err := llmagent.New(llmagent.Config{
		Name:        "the_agent",
		Model:       llm,
		Description: "A minimal learning agent.",
		Instruction: "You are a helpful assistant.",
		// Tools omitted: internal/dmrmodel has no tool-call conversion and
		// errors if Tools is set (see dmrmodel.GenerateContent).
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
