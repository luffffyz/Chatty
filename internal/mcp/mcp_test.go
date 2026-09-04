package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testServer 返回一个最小 Streamable HTTP MCP 服务器。
func testServer(t *testing.T, sse bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", "sess-1")
			resp := rpcReply(req.ID, `{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"test","version":"0"}}`)
			writeReply(w, resp, sse)
			return
		}
		if req.Method == "tools/list" {
			resp := rpcReply(req.ID, `{"tools":[{"name":"echo","description":"回显文本","inputSchema":{"type":"object","properties":{"text":{"type":"string"}}}}]}`)
			writeReply(w, resp, sse)
			return
		}
		if req.Method == "tools/call" {
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			var args map[string]string
			_ = json.Unmarshal(p.Arguments, &args)
			if p.Name == "echo" {
				resp := rpcReply(req.ID, `{"content":[{"type":"text","text":"`+args["text"]+`"}],"isError":false}`)
				writeReply(w, resp, sse)
				return
			}
		}
		writeReply(w, `{"jsonrpc":"2.0","id":`+idStr(req.ID)+`,"error":{"code":-32601,"message":"method not found"}}`, sse)
	})
	return httptest.NewServer(mux)
}

func idStr(id *int) string {
	if id == nil {
		return "null"
	}
	b, _ := json.Marshal(*id)
	return string(b)
}

func rpcReply(id *int, result string) string {
	return `{"jsonrpc":"2.0","id":` + idStr(id) + `,"result":` + result + `}`
}

func writeReply(w http.ResponseWriter, payload string, sse bool) {
	w.Header().Set("Content-Type", "application/json")
	if sse {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: " + payload + "\n\n"))
		return
	}
	_, _ = w.Write([]byte(payload))
}

func TestClientListAndCall(t *testing.T) {
	for _, sse := range []bool{false, true} {
		sv := testServer(t, sse)
		c := New(sv.URL+"/mcp", "")
		ctx := context.Background()

		tools, err := c.ListTools(ctx)
		if err != nil {
			t.Fatalf("sse=%v ListTools: %v", sse, err)
		}
		if len(tools) != 1 || tools[0].Name != "echo" {
			t.Fatalf("sse=%v tools = %+v", sse, tools)
		}

		res, err := c.CallTool(ctx, "echo", json.RawMessage(`{"text":"hi"}`))
		if err != nil {
			t.Fatalf("sse=%v CallTool: %v", sse, err)
		}
		if !strings.Contains(res.Content, "hi") {
			t.Fatalf("sse=%v call result = %q", sse, res.Content)
		}
		sv.Close()
	}
}

func TestClientHTTPError(t *testing.T) {
	sv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"denied"}}`, http.StatusForbidden)
	}))
	defer sv.Close()
	c := New(sv.URL, "")
	if _, err := c.ListTools(context.Background()); err == nil {
		t.Fatal("expected error for 403")
	}
}
