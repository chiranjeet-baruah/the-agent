package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chiranjeet14/the-agent/internal/agent"
	"github.com/chiranjeet14/the-agent/internal/httpapi"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// checkOllamaReachable does a quick GET against Ollama's OpenAI-compatible
// models endpoint so a misconfigured/stopped Ollama fails loudly at startup
// instead of the server silently 502ing on the first real request.
func checkOllamaReachable(baseURL string) error {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/models")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func main() {
	modelName := getenv("MODEL", "llama3.2")
	ollamaBaseURL := getenv("OLLAMA_BASE_URL", "http://localhost:11434/v1")
	ollamaAPIKey := getenv("OLLAMA_API_KEY", "ollama")
	port := getenv("PORT", "8080")

	if err := checkOllamaReachable(ollamaBaseURL); err != nil {
		log.Fatalf("Ollama not reachable at %s: %v\nIs `docker compose up -d` running and is the model pulled?", ollamaBaseURL, err)
	}

	ctx := context.Background()
	rnr, err := agent.New(ctx, agent.Config{
		ModelName:     modelName,
		OllamaBaseURL: ollamaBaseURL,
		OllamaAPIKey:  ollamaAPIKey,
	})
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	handler := &httpapi.Handler{Runner: rnr}

	router := gin.Default()
	router.POST("/chat", handler.Chat)
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	log.Printf("Listening on :%s (model=%s, ollama=%s)", port, modelName, ollamaBaseURL)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
