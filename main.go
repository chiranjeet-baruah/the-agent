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

// checkModelRunnerReachable does a quick GET against Docker Model Runner's
// OpenAI-compatible models endpoint so a disabled/misconfigured Model Runner
// fails loudly at startup instead of the server silently 502ing on the first
// real request.
func checkModelRunnerReachable(baseURL string) error {
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
	modelName := getenv("MODEL", "ai/llama3.2")
	modelRunnerBaseURL := getenv("MODEL_RUNNER_BASE_URL", "http://localhost:12434/engines/v1")
	port := getenv("PORT", "8080")

	if err := checkModelRunnerReachable(modelRunnerBaseURL); err != nil {
		log.Fatalf("Docker Model Runner not reachable at %s: %v\nIs `docker desktop enable model-runner --tcp 12434` done and is the model pulled?", modelRunnerBaseURL, err)
	}

	ctx := context.Background()
	rnr, err := agent.New(ctx, agent.Config{
		ModelName: modelName,
		BaseURL:   modelRunnerBaseURL,
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

	log.Printf("Listening on :%s (model=%s, model-runner=%s)", port, modelName, modelRunnerBaseURL)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
