package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

func TestBrowserCORSAndCSRFPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy, err := NewOriginPolicy("https://ui.example")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.Use(policy.CORSMiddleware([]string{"/api"}))
	group := engine.Group("/api")
	group.Use(policy.BrowserCSRFMiddleware())
	group.POST("/save", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	tests := []struct {
		name       string
		origin     string
		requested  bool
		wantStatus int
	}{
		{name: "allowlisted origin", origin: "https://ui.example", requested: true, wantStatus: http.StatusNoContent},
		{name: "same origin", origin: "http://panel.example", requested: true, wantStatus: http.StatusNoContent},
		{name: "malicious origin", origin: "https://evil.example", requested: true, wantStatus: http.StatusForbidden},
		{name: "missing requested-with", origin: "https://ui.example", wantStatus: http.StatusForbidden},
		{name: "API client without Origin", requested: true, wantStatus: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "http://panel.example/api/save", nil)
			request.Host = "panel.example"
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.requested {
				request.Header.Set("X-Requested-With", "XMLHttpRequest")
			}
			engine.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
			}
			if test.wantStatus == http.StatusNoContent && test.origin != "" {
				if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != test.origin {
					t.Fatalf("allow origin = %q", got)
				}
				if got := recorder.Header().Get("Access-Control-Expose-Headers"); got != "Content-Disposition" {
					t.Fatalf("expose headers = %q", got)
				}
			}
		})
	}

	preflight := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "http://panel.example/api/save", nil)
	request.Host = "panel.example"
	request.Header.Set("Origin", "https://ui.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type, X-Requested-With")
	engine.ServeHTTP(preflight, request)
	if preflight.Code != http.StatusNoContent || preflight.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("preflight = %d %#v", preflight.Code, preflight.Header())
	}
}

func TestOriginPolicyRejectsWildcardAndPaths(t *testing.T) {
	for _, value := range []string{"*", "https://*.example", "https://ui.example/app"} {
		if _, err := NewOriginPolicy(value); err == nil {
			t.Fatalf("NewOriginPolicy(%q) unexpectedly succeeded", value)
		}
	}
}

func TestBrowserTerminalWebSocketOriginPatterns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy, err := NewOriginPolicy("https://ui.example")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.Use(policy.CORSMiddleware([]string{"/api"}))
	engine.GET("/api/terminal", func(c *gin.Context) {
		conn, acceptErr := websocket.Accept(c.Writer, c.Request, browserWebSocketAcceptOptions(c))
		if acceptErr == nil {
			_ = conn.Close(websocket.StatusNormalClosure, "done")
		}
	})
	server := httptest.NewServer(engine)
	defer server.Close()
	wsURL := "ws" + server.URL[len("http"):] + "/api/terminal"

	conn, response, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://ui.example"}}})
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("allowlisted terminal origin failed: status=%d err=%v", status, err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "done")

	_, response, err = websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://evil.example"}}})
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("malicious terminal origin: response=%v err=%v", response, err)
	}
}
