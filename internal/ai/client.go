// Package ai implements the OpenAI-compatible chat clients for DeepSeek and
// Kimi (Moonshot). It streams text deltas and does not persist credentials.
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Provider string

const (
	DeepSeek Provider = "deepseek"
	Kimi     Provider = "kimi"
)

type Config struct {
	Provider Provider
	APIKey   string
	BaseURL  string
	Model    string
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCall struct {
	Index    int    `json:"-"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ChatResult struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
}

func Defaults(provider Provider) (baseURL, model string) {
	switch provider {
	case Kimi:
		return "https://api.moonshot.cn/v1", "moonshot-v1-8k"
	default:
		return "https://api.deepseek.com/v1", "deepseek-chat"
	}
}

func Normalize(config Config) Config {
	if config.Provider != Kimi {
		config.Provider = DeepSeek
	}
	base, model := Defaults(config.Provider)
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = base
	}
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if strings.TrimSpace(config.Model) == "" {
		config.Model = model
	}
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.Model = strings.TrimSpace(config.Model)
	return config
}

type Client struct {
	config Config
	http   *http.Client
}

func New(config Config) *Client {
	return &Client{config: Normalize(config), http: &http.Client{Timeout: 2 * time.Minute}}
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
}

type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content          string          `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			ToolCalls        []toolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *Client) endpoint() string {
	if strings.HasSuffix(c.config.BaseURL, "/chat/completions") {
		return c.config.BaseURL
	}
	return c.config.BaseURL + "/chat/completions"
}

type toolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (c *Client) Stream(ctx context.Context, messages []Message, onDelta func(string) error) error {
	_, err := c.ChatStream(ctx, messages, nil, onDelta, nil)
	return err
}

// ChatStream sends a streaming chat completion, reports content and reasoning
// deltas, and returns the complete text plus any tool calls the model asked for.
func (c *Client) ChatStream(ctx context.Context, messages []Message, tools []Tool, onDelta, onReasoning func(string) error) (ChatResult, error) {
	if c.config.APIKey == "" {
		return ChatResult{}, errors.New("尚未配置 AI API Key")
	}
	request := chatRequest{
		Model:    c.config.Model,
		Messages: messages,
		Stream:   true,
		Tools:    tools,
	}
	if len(tools) > 0 {
		request.ToolChoice = "auto"
	}
	body, err := json.Marshal(request)
	if err != nil {
		return ChatResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return ChatResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChatResult{}, responseError(resp)
	}

	var content strings.Builder
	var reasoning strings.Builder
	toolCalls := make(map[int]*ToolCall)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return finishChatResult(content.String(), reasoning.String(), toolCalls), nil
		}
		var chunk chatChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return ChatResult{}, errors.New(chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			delta = chunk.Choices[0].Message.Content
		}
		if delta != "" {
			content.WriteString(delta)
			if onDelta != nil {
				if err := onDelta(delta); err != nil {
					return ChatResult{}, err
				}
			}
		}
		reasoningDelta := chunk.Choices[0].Delta.ReasoningContent
		if reasoningDelta == "" {
			reasoningDelta = chunk.Choices[0].Message.ReasoningContent
		}
		if reasoningDelta != "" {
			reasoning.WriteString(reasoningDelta)
			if onReasoning != nil {
				if err := onReasoning(reasoningDelta); err != nil {
					return ChatResult{}, err
				}
			}
		}
		for _, item := range chunk.Choices[0].Delta.ToolCalls {
			call := toolCalls[item.Index]
			if call == nil {
				call = &ToolCall{Index: item.Index}
				toolCalls[item.Index] = call
			}
			if item.ID != "" {
				call.ID = item.ID
			}
			if item.Type != "" {
				call.Type = item.Type
			}
			if item.Function.Name != "" {
				call.Function.Name = item.Function.Name
			}
			call.Function.Arguments += item.Function.Arguments
		}
	}
	if err := scanner.Err(); err != nil {
		return ChatResult{}, err
	}
	return finishChatResult(content.String(), reasoning.String(), toolCalls), nil
}

func finishChatResult(content, reasoning string, calls map[int]*ToolCall) ChatResult {
	result := ChatResult{Content: content, Reasoning: reasoning}
	indexes := make([]int, 0, len(calls))
	for index := range calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		call := *calls[index]
		if call.Type == "" {
			call.Type = "function"
		}
		if call.ID == "" {
			call.ID = "call_" + strconv.Itoa(index)
		}
		result.ToolCalls = append(result.ToolCalls, call)
	}
	return result
}

func (c *Client) Test(ctx context.Context) error {
	if c.config.APIKey == "" {
		return errors.New("尚未配置 AI API Key")
	}
	body, err := json.Marshal(chatRequest{
		Model:     c.config.Model,
		Messages:  []Message{{Role: "user", Content: "ping"}},
		MaxTokens: 1,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp)
	}
	return nil
}

func responseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &parsed) == nil && parsed.Error.Message != "" {
		return fmt.Errorf("AI 接口返回 %s: %s", resp.Status, parsed.Error.Message)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return fmt.Errorf("AI 接口返回 %s", resp.Status)
	}
	return fmt.Errorf("AI 接口返回 %s: %s", resp.Status, text)
}
