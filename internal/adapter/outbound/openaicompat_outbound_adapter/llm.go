package openaicompat_outbound_adapter

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// llmModel implements google.golang.org/adk/v2/model.LLM against an
// OpenAI-compatible Chat Completions API.
//
// adk-go's own model/openaimodel package can't be reused here: it's
// hardcoded to call the OpenAI Responses API, which not every
// OpenAI-compatible backend implements (Docker Model Runner, for example,
// only exposes /chat/completions, /completions, /embeddings, /models).
type llmModel struct {
	client *openai.Client
	name   string
}

// newLLMModel builds an llmModel. The configured backend's API is assumed
// unauthenticated, so no API key handling is needed.
func newLLMModel(name, baseURL string) *llmModel {
	client := openai.NewClient(option.WithBaseURL(baseURL))
	return &llmModel{client: &client, name: name}
}

func (m *llmModel) Name() string { return m.name }

func (m *llmModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if stream {
			yield(nil, errors.New("openaicompat_outbound_adapter: streaming not supported"))
			return
		}
		if len(req.Tools) > 0 {
			// Fail loudly: this adapter has no tool-call conversion.
			// Silently ignoring req.Tools would mean the model simply
			// never calls the tool, with no error anywhere.
			yield(nil, errors.New("openaicompat_outbound_adapter: tool calling not supported"))
			return
		}

		var messages []openai.ChatCompletionMessageParamUnion
		if req.Config != nil && req.Config.SystemInstruction != nil {
			messages = append(messages, openai.SystemMessage(contentText(req.Config.SystemInstruction)))
		}
		for _, c := range req.Contents {
			text := contentText(c)
			if c.Role == genai.RoleModel {
				messages = append(messages, openai.AssistantMessage(text))
			} else {
				messages = append(messages, openai.UserMessage(text))
			}
		}

		resp, err := m.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(m.name),
			Messages: messages,
		})
		if err != nil {
			yield(nil, fmt.Errorf("openaicompat_outbound_adapter: call failed: %w", err))
			return
		}
		if len(resp.Choices) == 0 {
			yield(nil, errors.New("openaicompat_outbound_adapter: no choices returned"))
			return
		}

		yield(&model.LLMResponse{
			Content: genai.NewContentFromText(resp.Choices[0].Message.Content, genai.RoleModel),
		}, nil)
	}
}

func contentText(c *genai.Content) string {
	var sb strings.Builder
	for _, p := range c.Parts {
		sb.WriteString(p.Text)
	}
	return sb.String()
}
