// Command the-agent is the composition root: it wires the outbound adapter,
// domain, and inbound adapter together and starts the HTTP server.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	gininboundadapter "github.com/chiranjeet14/the-agent/internal/adapter/inbound/gin_inbound_adapter"
	dockermodelrunneroutboundadapter "github.com/chiranjeet14/the-agent/internal/adapter/outbound/dockermodelrunner_outbound_adapter"
	"github.com/chiranjeet14/the-agent/internal/config"
	"github.com/chiranjeet14/the-agent/internal/domain"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	engine, err := dockermodelrunneroutboundadapter.New(dockermodelrunneroutboundadapter.Config{
		ModelName: cfg.ModelName,
		BaseURL:   cfg.ModelRunnerBaseURL,
	})
	if err != nil {
		log.Fatalf("Failed to initialize agent engine: %v", err)
	}

	ctx := context.Background()
	if err := engine.Ping(ctx); err != nil {
		log.Fatalf("Docker Model Runner not reachable at %s: %v\nIs `docker desktop enable model-runner --tcp=12434` done and is the model pulled?", cfg.ModelRunnerBaseURL, err)
	}

	chatDomain := &domain.ChatDomain{Engine: engine}
	handler := &gininboundadapter.Handler{Chat: chatDomain}

	router := gin.Default()
	router.POST("/chat", handler.HandleChat)
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	log.Printf("Listening on :%s (model=%s, model-runner=%s)", cfg.Port, cfg.ModelName, cfg.ModelRunnerBaseURL)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
