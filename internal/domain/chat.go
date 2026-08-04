// Package domain holds business logic independent of any external
// framework or technology (no gin, no adk-go, no HTTP types).
package domain

import (
	"context"
	"time"

	"github.com/chiranjeet14/the-agent/internal/port/inbound"
	"github.com/chiranjeet14/the-agent/internal/port/outbound"
)

// runTimeout bounds how long a single chat turn waits on the agent engine
// before failing, so a hung backend can't hang the request forever.
const runTimeout = 60 * time.Second

// localUserID is a placeholder: this is a single-user learning project,
// no auth.
const localUserID = "local-user"

// ChatDomain implements inbound.ChatPort using an outbound.AgentEnginePort.
type ChatDomain struct {
	Engine outbound.AgentEnginePort
}

var _ inbound.ChatPort = (*ChatDomain)(nil)

func (d *ChatDomain) Chat(ctx context.Context, sessionID, message string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	return d.Engine.RunTurn(ctx, localUserID, sessionID, message)
}
