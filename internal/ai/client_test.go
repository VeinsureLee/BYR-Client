package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := New(Config{Provider: DeepSeek, APIKey: "test-key", BaseURL: server.URL + "/v1", Model: "deepseek-chat"})
	var output strings.Builder
	if err := client.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(delta string) error {
		output.WriteString(delta)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "你好" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestStreamErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer server.Close()
	client := New(Config{Provider: Kimi, APIKey: "bad", BaseURL: server.URL + "/v1", Model: "moonshot-v1-8k"})
	err := client.Test(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error = %v", err)
	}
}

func TestDefaultsAndNormalize(t *testing.T) {
	base, model := Defaults(Kimi)
	if base != "https://api.moonshot.cn/v1" || model != "moonshot-v1-8k" {
		t.Fatalf("Kimi defaults = %s %s", base, model)
	}
	config := Normalize(Config{Provider: Kimi, BaseURL: " https://example.com/v1/ "})
	if config.BaseURL != "https://example.com/v1" || config.Model != "moonshot-v1-8k" {
		t.Fatalf("normalized = %+v", config)
	}
}

func TestChatStreamToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"search_forum\",\"arguments\":\"{\\\"keywords\\\"\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\":[\\\"Go\\\"]}\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	client := New(Config{Provider: DeepSeek, APIKey: "test", BaseURL: server.URL + "/v1", Model: "deepseek-chat"})
	result, err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "search"}}, []Tool{{
		Type:     "function",
		Function: ToolFunction{Name: "search_forum", Description: "search", Parameters: []byte(`{"type":"object"}`)},
	}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v", result.ToolCalls)
	}
	call := result.ToolCalls[0]
	if call.ID != "call_1" || call.Function.Name != "search_forum" || call.Function.Arguments != `{"keywords":["Go"]}` {
		t.Fatalf("call = %+v", call)
	}
}

func TestChatStreamReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"先分析\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"结论\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	client := New(Config{Provider: DeepSeek, APIKey: "test", BaseURL: server.URL + "/v1", Model: "deepseek-reasoner"})
	var reasoning strings.Builder
	result, err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil, nil, func(delta string) error {
		reasoning.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if reasoning.String() != "先分析" || result.Reasoning != "先分析" || result.Content != "结论" {
		t.Fatalf("reasoning=%q result=%+v", reasoning.String(), result)
	}
}

func TestKimiRequestOmitsTemperature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, exists := body["temperature"]; exists {
			t.Errorf("temperature was sent to Kimi: %+v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	client := New(Config{Provider: Kimi, APIKey: "test", BaseURL: server.URL + "/v1", Model: "moonshot-v1-8k"})
	if _, err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
}
