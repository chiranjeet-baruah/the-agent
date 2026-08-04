// Package outbound defines secondary ports: what the domain needs from
// external systems, implemented by outbound adapters.
package outbound

import "context"

// AgentEnginePort is what the domain needs from an LLM/agent engine.
type AgentEnginePort interface {
	// RunTurn runs one turn of the agent for the given user/session and
	// returns its reply text.
	RunTurn(ctx context.Context, userID, sessionID, message string) (string, error)
	// Ping checks that the engine's backend is reachable.
	Ping(ctx context.Context) error
}
