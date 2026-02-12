package provider

import (
	"context"
	"encoding/json"
)

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	Chat(ctx context.Context, params ChatParams) (*ChatResponse, error)
}

type ChatParams struct {
	Model     string
	System    string
	Messages  []Message
	Tools     []ToolDefinition
	MaxTokens int
}

type Message struct {
	Role    string         // "user" or "assistant"
	Content []ContentBlock // For multi-block messages (text + tool_use + tool_result)
}

type ContentBlock struct {
	Type string // "text", "tool_use", "tool_result"

	// For text blocks
	Text string

	// For tool_use blocks
	ToolUseID string
	ToolName  string
	Input     json.RawMessage

	// For tool_result blocks
	ToolUseIDRef string
	Content      string
	IsError      bool
}

type ChatResponse struct {
	Content    []ContentBlock
	StopReason string // "end_turn", "tool_use", "max_tokens"
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema ToolInputSchema
}

type ToolInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required,omitempty"`
}

// Helper constructors

func NewTextBlock(text string) ContentBlock {
	return ContentBlock{Type: "text", Text: text}
}

func NewToolUseBlock(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{Type: "tool_use", ToolUseID: id, ToolName: name, Input: input}
}

func NewToolResultBlock(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{Type: "tool_result", ToolUseIDRef: toolUseID, Content: content, IsError: isError}
}

func NewUserMessage(blocks ...ContentBlock) Message {
	return Message{Role: "user", Content: blocks}
}

func NewAssistantMessage(blocks ...ContentBlock) Message {
	return Message{Role: "assistant", Content: blocks}
}
