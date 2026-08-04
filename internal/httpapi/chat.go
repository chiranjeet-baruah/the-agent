// Package httpapi exposes the agent over HTTP.
package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/genai"
)

// runTimeout bounds how long a single /chat request waits on the model
// before failing, so a hung Ollama can't hang the HTTP request forever.
const runTimeout = 60 * time.Second

// localUserID is a placeholder: increment 1 is single-user, no auth.
const localUserID = "local-user"

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

// Handler serves the agent over HTTP.
type Handler struct {
	Runner *runner.Runner
}

// Chat handles POST /chat: runs one turn of the agent and returns its reply.
func (h *Handler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), runTimeout)
	defer cancel()

	msg := genai.NewContentFromText(req.Message, genai.RoleUser)

	var reply strings.Builder
	for ev, err := range h.Runner.Run(ctx, localUserID, req.SessionID, msg, agent.RunConfig{}) {
		if err != nil {
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: "agent run failed: " + err.Error()})
			return
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
	c.JSON(http.StatusOK, ChatResponse{Reply: reply.String()})
}
