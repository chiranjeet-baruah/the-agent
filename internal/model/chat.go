// Package model holds plain data-transfer structures (entities, requests,
// responses) — no logic, no dependencies on adapters or the domain.
package model

type ChatRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
}

type ChatResponse struct {
	Reply string `json:"reply"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
