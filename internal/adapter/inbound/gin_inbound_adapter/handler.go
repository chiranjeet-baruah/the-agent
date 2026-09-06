// Package gin_inbound_adapter adapts HTTP (gin) requests to the ChatPort.
package gin_inbound_adapter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/chiranjeet-baruah/the-agent/internal/model"
	"github.com/chiranjeet-baruah/the-agent/internal/port/inbound"
)

// Handler drives inbound.ChatPort from HTTP requests.
type Handler struct {
	Chat inbound.ChatPort
}

// HandleChat handles POST /chat: runs one turn of the agent and returns its
// reply.
func (h *Handler) HandleChat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	reply, err := h.Chat.Chat(c.Request.Context(), req.SessionID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadGateway, model.ErrorResponse{Error: "agent run failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.ChatResponse{Reply: reply})
}
