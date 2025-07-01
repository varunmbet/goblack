package goblack

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

type TestController struct{}

func (t *TestController) Hello(c *Context) {
	c.String(200, "hello controller")
}

var app *Application

func init() {
	app = Instance("demo")
	app.Router().RegisterController(&TestController{})
	app.Router().Get("/ping", func(c *Context) {
		c.String(200, "pong")
	})
	app.Router().Get("/hello", "TestController@Hello")
	app.Router().Get("/user/:id", func(c *Context) {
		c.String(200, c.Param("id"))
	})
	app.Router().Get("/group", func(c *Context) {
		c.String(200, "group")
	})
}

func TestHandlerFuncRoute(t *testing.T) {
	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "pong") {
		t.Errorf("expect pong, got %s", w.Body.String())
	}
}

func TestControllerStringRoute(t *testing.T) {
	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "hello controller") {
		t.Errorf("expect hello controller, got %s", w.Body.String())
	}
}

func TestDynamicParamRoute(t *testing.T) {
	req := httptest.NewRequest("GET", "/user/123", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "123") {
		t.Errorf("expect 123, got %s", w.Body.String())
	}
}

func TestNotFoundRoute(t *testing.T) {
	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != 404 && !strings.Contains(w.Body.String(), "Not Found") {
		t.Errorf("expect 404 Not Found, got %d %s", w.Code, w.Body.String())
	}
}

func TestGroupRoute(t *testing.T) {
	called := false
	app.Router().Group("/api", func(c *Context) {
		called = true
		c.Next()
	})
	app.Router().Get("/data", func(c *Context) {
		c.String(200, "ok")
	})
	app.Router().EndGroup()

	req := httptest.NewRequest("GET", "/api/data", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if !called {
		t.Errorf("group middleware not called")
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("expect ok, got %s", w.Body.String())
	}
}

func TestGlobalMiddleware(t *testing.T) {
	called := false
	app.Router().Use(func(c *Context) {
		called = true
		c.Next()
	})
	app.Router().Get("/mwtest", func(c *Context) {
		c.String(200, "mw ok")
	})

	req := httptest.NewRequest("GET", "/mwtest", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if !called {
		t.Errorf("global middleware not called")
	}
	if !strings.Contains(w.Body.String(), "mw ok") {
		t.Errorf("expect mw ok, got %s", w.Body.String())
	}
}

func TestWebsocket(t *testing.T) {
	app.Websocket("/ws", func(conn *websocket.Conn, c *Context) {
		conn.WriteMessage(websocket.TextMessage, []byte("hello ws"))
	})
	ts := httptest.NewServer(app)
	defer ts.Close()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("websocket read error: %v", err)
	}
	if string(msg) != "hello ws" {
		t.Errorf("expect hello ws, got %s", string(msg))
	}
	ws.Close()
}

func TestGroupAndGlobalMiddleware(t *testing.T) {
	globalCalled := false
	groupCalled := false

	app.Router().Use(func(c *Context) {
		globalCalled = true
		c.Next()
	})

	app.Router().Group("/api", func(c *Context) {
		groupCalled = true
		c.Next()
	})
	app.Router().Get("/mix", func(c *Context) {
		c.String(200, "mix ok")
	})
	app.Router().EndGroup()

	// Group外部路由
	app.Router().Get("/out", func(c *Context) {
		c.String(200, "out ok")
	})

	// 测试分组内路由
	req := httptest.NewRequest("GET", "/api/mix", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if !globalCalled {
		t.Errorf("global middleware not called")
	}
	if !groupCalled {
		t.Errorf("group middleware not called")
	}
	if !strings.Contains(w.Body.String(), "mix ok") {
		t.Errorf("expect mix ok, got %s", w.Body.String())
	}

	// 测试分组外路由
	globalCalled = false
	groupCalled = false
	req2 := httptest.NewRequest("GET", "/out", nil)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if !globalCalled {
		t.Errorf("global middleware not called for /out")
	}
	if groupCalled {
		t.Errorf("group middleware should not be called for /out")
	}
	if !strings.Contains(w2.Body.String(), "out ok") {
		t.Errorf("expect out ok, got %s", w2.Body.String())
	}
}
