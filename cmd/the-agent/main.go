// Command the-agent is the composition root: it wires the outbound adapter,
// domain, and inbound adapter together and starts the HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	gininboundadapter "github.com/chiranjeet-baruah/the-agent/internal/adapter/inbound/gin_inbound_adapter"
	openaicompatoutboundadapter "github.com/chiranjeet-baruah/the-agent/internal/adapter/outbound/openaicompat_outbound_adapter"
	"github.com/chiranjeet-baruah/the-agent/internal/config"
	"github.com/chiranjeet-baruah/the-agent/internal/domain"
)

// shutdownTimeout bounds how long the server waits for in-flight requests to
// finish on shutdown before forcing the connections closed.
const shutdownTimeout = 65 * time.Second

func main() {
	candidates, port, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()
	engine, provider, model, baseURL, err := selectEngine(ctx, candidates)
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Printf("Using provider %s (model=%s, backend=%s)", provider, model, baseURL)

	chatDomain := &domain.ChatDomain{Engine: engine}
	handler := &gininboundadapter.Handler{Chat: chatDomain}

	router := gin.Default()
	router.POST("/chat", handler.HandleChat)
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	srv := &http.Server{Addr: ":" + port, Handler: router, ReadHeaderTimeout: 5 * time.Second}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("Listening on :%s", port)
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

// selectEngine builds an engine for the first candidate whose backend is
// reachable and has a usable model, in order. Candidates after the first
// only matter when config.Load returned more than one (auto-selection, no
// explicit provider/PROVIDER set) — an explicit selection always yields
// exactly one. Fetching each candidate's live model list (to resolve which
// model to use) doubles as its reachability/auth check, so there's no
// separate Ping call here — Ping remains available on the built Adapter for
// other uses.
func selectEngine(ctx context.Context, candidates []config.Candidate) (*openaicompatoutboundadapter.Adapter, string, string, string, error) {
	var failures []string
	for _, c := range candidates {
		live, err := openaicompatoutboundadapter.ListModels(ctx, c.BaseURL, c.APIKey)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", c.Provider, err))
			continue
		}
		resolved, err := resolveModel(c, live)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", c.Provider, err))
			continue
		}
		eng, err := openaicompatoutboundadapter.New(openaicompatoutboundadapter.Config{
			ModelName: resolved,
			BaseURL:   c.BaseURL,
			APIKey:    c.APIKey,
		})
		if err != nil {
			return nil, "", "", "", fmt.Errorf("initialize agent engine for provider %s: %w", c.Provider, err)
		}
		return eng, c.Provider, resolved, c.BaseURL, nil
	}
	return nil, "", "", "", fmt.Errorf("no configured provider is reachable:\n%s", strings.Join(failures, "\n"))
}

// resolveModel picks which model to use for a candidate given its backend's
// live model list. An explicit override (only possible via explicit
// provider selection) wins outright, unvalidated — the operator asked for
// it by name. Otherwise, the first of the candidate's config.yaml model
// preferences that's actually live wins; if none are (a provider's live
// list can include non-chat models config.yaml never listed), that's a
// failure rather than a guess, naming the live models so config.yaml can be
// corrected.
func resolveModel(c config.Candidate, live []string) (string, error) {
	if c.ModelOverride != "" {
		return c.ModelOverride, nil
	}
	for _, pref := range c.ModelPreferences {
		if slices.Contains(live, pref) {
			return pref, nil
		}
	}
	return "", fmt.Errorf("none of the configured models (%s) are available; live models: %s",
		strings.Join(c.ModelPreferences, ", "), strings.Join(live, ", "))
}
