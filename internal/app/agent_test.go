package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"BYR-Client/internal/ai"
)

func TestAIToolsSearchAndRead(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()

	value, _, _, _, err := a.runAITool(ctx, "search_forum", `{"keywords":["Go"],"time_days":0}`)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(value)
	var search struct {
		Topics []struct {
			Title     string `json:"title"`
			BoardID   string `json:"board_id"`
			ArticleID string `json:"article_id"`
			URL       string `json:"url"`
		} `json:"topics"`
	}
	if err := json.Unmarshal(data, &search); err != nil {
		t.Fatal(err)
	}
	if len(search.Topics) == 0 {
		t.Fatalf("search result = %s", data)
	}
	topic := search.Topics[0]
	if topic.Title == "" || topic.BoardID == "" || topic.ArticleID == "" {
		t.Fatalf("topic = %+v", topic)
	}

	readArgs, _ := json.Marshal(map[string]any{
		"board_id": topic.BoardID, "article_id": topic.ArticleID, "max_replies": 3,
	})
	value, _, _, _, err = a.runAITool(ctx, "read_thread", string(readArgs))
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(value)
	var thread struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Replies []any  `json:"replies"`
	}
	if err := json.Unmarshal(data, &thread); err != nil {
		t.Fatal(err)
	}
	if thread.Title == "" || thread.Content == "" || len(thread.Replies) > 3 {
		t.Fatalf("thread = %+v", thread)
	}
}

func TestAIToolsCatalogTrafficAndLatest(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()
	for _, call := range []struct {
		name string
		args string
	}{
		{"list_boards", `{"query":"Go","limit":10}`},
		{"get_traffic", `{"node":"技术"}`},
		{"latest_topic", `{"node":"技术"}`},
	} {
		if _, _, _, _, err := a.runAITool(ctx, call.name, call.args); err != nil {
			t.Fatalf("%s: %v", call.name, err)
		}
	}
}

func TestChatExecutesToolAndContinues(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		if n == 1 {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"search_forum\",\"arguments\":\"{\\\"keywords\\\":[\\\"Go\\\"],\\\"time_days\\\":0}\"}}]}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"已找到相关帖子\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	a, _ := newTestApp(t)
	config := ai.Config{Provider: ai.DeepSeek, APIKey: "test", BaseURL: server.URL + "/v1", Model: "deepseek-chat"}
	var output strings.Builder
	var events []ToolEvent
	answer, err := a.Chat(context.Background(), config, "有哪些 Go 帖子", ChatOptions{Scope: "全站", Time: "近一个月", Total: 50, PerBoard: 10}, func(delta string) error {
		output.WriteString(delta)
		return nil
	}, nil, func(event ToolEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer, "已找到相关帖子") || !strings.Contains(output.String(), "已找到相关帖子") {
		t.Fatalf("answer = %q output = %q", answer, output.String())
	}
	if requests.Load() < 2 {
		t.Fatalf("requests = %d", requests.Load())
	}
	completed := false
	for _, event := range events {
		if event.Name == "search_forum" && event.Phase == "completed" {
			completed = true
			if event.Arguments == "" {
				t.Fatal("tool event lost arguments")
			}
			if len(event.Citations) == 0 {
				t.Fatal("tool event lost citations")
			}
		}
	}
	if !completed {
		t.Fatalf("events = %+v", events)
	}
}

func TestChatDeduplicatesRepeatedToolCalls(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		switch n {
		case 1, 2:
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"search_forum\",\"arguments\":\"{\\\"keywords\\\":[\\\"Go\\\"],\\\"time_days\\\":0}\"}}]}}]}\n\n")
		default:
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"基于已有搜索结果回答\"}}]}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	a, _ := newTestApp(t)
	config := ai.Config{Provider: ai.DeepSeek, APIKey: "test", BaseURL: server.URL + "/v1", Model: "deepseek-chat"}
	var events []ToolEvent
	answer, err := a.Chat(context.Background(), config, "有哪些 Go 帖子", ChatOptions{Scope: "全站", Time: "不限"}, nil, nil, func(event ToolEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer, "基于已有搜索结果回答") {
		t.Fatalf("answer = %q", answer)
	}
	cached := false
	for _, event := range events {
		if event.Summary == "相同调用结果已复用" {
			cached = true
		}
	}
	if !cached {
		t.Fatalf("duplicate tool call was not reused: %+v", events)
	}
}

func TestChatForcesFinalAnswerAfterToolBudget(t *testing.T) {
	var rounds atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Tools []json.RawMessage `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "text/event-stream")
		if len(request.Tools) == 0 {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"最终回答\"}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		n := rounds.Add(1)
		args := fmt.Sprintf(`{"keywords":["Go"],"time_days":%d}`, n)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_%d\",\"type\":\"function\",\"function\":{\"name\":\"search_forum\",\"arguments\":%q}}]}}]}\n\n", n, args)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	a, _ := newTestApp(t)
	config := ai.Config{Provider: ai.DeepSeek, APIKey: "test", BaseURL: server.URL + "/v1", Model: "deepseek-chat"}
	answer, err := a.Chat(context.Background(), config, "全站搜索", ChatOptions{Scope: "全站", Time: "不限"}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer, "最终回答") {
		t.Fatalf("answer = %q", answer)
	}
	if rounds.Load() != maxToolRounds {
		t.Fatalf("tool rounds = %d, want %d", rounds.Load(), maxToolRounds)
	}
}
