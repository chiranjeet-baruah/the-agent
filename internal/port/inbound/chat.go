// Package inbound defines primary ports: the use cases inbound adapters
// (e.g. HTTP) can invoke on the domain.
package inbound

import "context"

// ChatPort is the use case an inbound adapter drives: run one chat turn.
type ChatPort interface {
	Chat(ctx context.Context, sessionID, message string) (string, error)
}
