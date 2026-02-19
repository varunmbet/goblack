# GoBlack

GoBlack is a lightweight web framework in Go, with a trie-based router, middleware chain, route grouping, controller-string routing, and websocket helpers.

## Features

- Trie router with static, param (`:id`) and catch-all (`*path`) routes
- Two route styles:
  - Controller string: `"module@action"`
  - Direct handler: `func(*Context)` / `HandlerFunc`
- Global middleware + group middleware
- Group scope with `Group(...)` / `EndGroup()`
- HTTP + websocket support
- Context helper methods for params, response, render, session

## Install

```bash
go get github.com/varunmbet/goblack
```

## Quick Start

```go
package main

import "github.com/varunmbet/goblack"

func main() {
	app := goblack.Default()

	app.Router().Get("/ping", func(c *goblack.Context) {
		c.String(200, "pong")
	})

	app.Run(":8080")
}
```

## Routing

### 1) Direct handler style

```go
app.Router().Get("/hello", func(c *goblack.Context) {
	c.String(200, "hello")
})
```

### 2) Controller string style

```go
app.Router().Get("/api/captcha", "admin_auth@captcha")
```

Important for controller-string style:

1. Register controller methods in controller package:

```go
func init() {
	goblack.Instance("demo").Router().RegisterController(&Admin_admin{})
}
```

2. Ensure the controller package is imported (for side-effect init):

```go
import _ "gocms/app/controllers/admin"
```

If not imported, methods are never registered and route lookup falls back to Not Found.

## Middleware

### Global middleware

```go
app.Router().Use(func(c *goblack.Context) {
	// before
	c.Next()
	// after
})
```

### Group middleware

```go
app.Router().Group("/api", func(c *goblack.Context) {
	c.Next()
})

app.Router().Get("/users", func(c *goblack.Context) {
	c.String(200, "ok")
})

app.Router().EndGroup()
```

Route becomes `/api/users`.

## Websocket

```go
app.Websocket("/ws", func(conn *websocket.Conn, c *goblack.Context) {
	_ = conn.WriteMessage(websocket.TextMessage, []byte("hello ws"))
})
```

## Route Debugging

Use:

```go
app.Router().Show()
```

It prints each registered route with HTTP method and binding info.

## Testing

- Keep tests independent; avoid sharing mutable global router state across test cases.
- Prefer creating a fresh app/router per test.
- If using a global app in tests, clean up route/middleware side effects carefully.

## Notes

- `Router.String()` now safely returns `{}` if GET tree is empty.
- Middleware supports `HandlerFunc`, `func(*Context)`, `http.Handler`, `http.HandlerFunc`, and `func(http.ResponseWriter, *http.Request)`.
