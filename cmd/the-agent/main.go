// Command the-agent is the composition root: it wires the outbound adapter,
// domain, and inbound adapter together and starts the HTTP server.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	gininboundadapter "github.com/chiranjeet14/the-agent/internal/adapter/inbound/gin_inbound_adapter"
	openaicompatoutboundadapter "github.com/chiranjeet14/the-agent/internal/adapter/outbound/openaicompat_outbound_adapter"
	"github.com/chiranjeet14/the-agent/internal/config"
	"github.com/chiranjeet14/the-agent/internal/domain"
)

// shutdownTimeout bounds how long the server waits for in-flight requests to
// finish on shutdown before forcing the connections closed.
const shutdownTimeout = 65 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	engine, err := openaicompatoutboundadapter.New(openaicompatoutboundadapter.Config{
		ModelName: cfg.ModelName,
		BaseURL:   cfg.BaseURL,
	})
	if err != nil {
		log.Fatalf("Failed to initialize agent engine: %v", err)
	}

	ctx := context.Background()
	if err := engine.Ping(ctx); err != nil {
		log.Fatalf("LLM backend not reachable at %s: %v\nIs it running and is the model pulled?", cfg.BaseURL, err)
	}

	chatDomain := &domain.ChatDomain{Engine: engine}
	handler := &gininboundadapter.Handler{Chat: chatDomain}

	router := gin.Default()
	router.POST("/chat", handler.HandleChat)
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("Listening on :%s (model=%s, backend=%s)", cfg.Port, cfg.ModelName, cfg.BaseURL)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	case <-shutdownCtx.Done():
		log.Print("Shutting down...")
		stop()

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Graceful shutdown failed: %v", err)
		}
		log.Print("Shutdown complete")
	}
}
