package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicProvider struct {
	client *anthropic.Client
}

func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	client := anthropic.NewClient(opts...)
	return &AnthropicProvider{client: &client}
}

func (p *AnthropicProvider) Chat(ctx context.Context, params ChatParams) (*ChatResponse, error) {
	messages := make([]anthropic.MessageParam, 0, len(params.Messages))
	for _, msg := range params.Messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Content))
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				blocks = append(blocks, anthropic.NewTextBlock(block.Text))
			case "tool_use":
				var input any
				if err := json.Unmarshal(block.Input, &input); err != nil {
					input = map[string]any{}
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(block.ToolUseID, input, block.ToolName))
			case "tool_result":
				blocks = append(blocks, anthropic.NewToolResultBlock(block.ToolUseIDRef, block.Content, block.IsError))
			}
		}
		switch msg.Role {
		case "user":
			messages = append(messages, anthropic.NewUserMessage(blocks...))
		case "assistant":
			messages = append(messages, anthropic.NewAssistantMessage(blocks...))
		}
	}

	tools := make([]anthropic.ToolUnionParam, 0, len(params.Tools))
	for _, t := range params.Tools {
		tool := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: t.InputSchema.Properties,
				Required:   t.InputSchema.Required,
			},
		}
		tools = append(tools, anthropic.ToolUnionParam{OfTool: &tool})
	}

	maxTokens := int64(params.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(params.Model),
		MaxTokens: maxTokens,
		Messages:  messages,
		Tools:     tools,
		System: []anthropic.TextBlockParam{
			{Text: params.System},
		},
	}

	resp, err := p.client.Messages.New(ctx, apiParams)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	result := &ChatResponse{
		StopReason: string(resp.StopReason),
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			tb := block.AsText()
			result.Content = append(result.Content, NewTextBlock(tb.Text))
		case "tool_use":
			tb := block.AsToolUse()
			result.Content = append(result.Content, NewToolUseBlock(tb.ID, tb.Name, json.RawMessage(tb.Input)))
		}
	}

	return result, nil
}
