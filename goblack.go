package goblack

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
)

const (
	// DEV mode
	DEV = "development"
	// PROD mode
	PROD = "production"
	// TEST mode
	TEST = "test"
)

// Env default application runtime environment
var Env string

// Middleware middleware handler
type Middleware interface{}

// HandlerFunc context handler func
type HandlerFunc func(*Context)

// ErrorHandleFunc HTTP error handleFunc
type ErrorHandleFunc func(error, *Context)

type Application struct {
	debug           bool
	name            string
	di              DIer
	router          *Router
	pool            sync.Pool
	errorHandler    ErrorHandleFunc
	notFoundHandler HandlerFunc
	sessioopen      bool
	provider        IProvider
}

var appInstances map[string]*Application

// defaultAppName default application name
const defaultAppName = "_default_"

var upgrader = websocket.Upgrader{
	ReadBufferSize:    4096,
	WriteBufferSize:   4096,
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// New create a baa application without any config.
// 不用直接用这个函数，用 Instance 或者 Default
func NewApplication(name ...string) *Application {
	appname := defaultAppName
	if len(name) > 0 {
		appname = name[0]
	}
	b := new(Application)
	b.name = appname
	//b.middleware = make([]HandlerFunc, 0)
	b.pool = sync.Pool{
		New: func() interface{} {
			return NewContext(nil, nil, b)
		},
	}
	if Env != PROD {
		b.debug = true
	}
	b.sessioopen = false
	b.SetDIer(NewDI())
	b.SetDI("router", NewRouter())
	b.SetDI("logger", log.New(os.Stderr, "[GoBlack Application] ", log.LstdFlags))
	b.SetDI("render", newRender())
	b.SetDI("validator", validator.New())
	b.SetNotFound(b.DefaultNotFoundHandler)
	return b
}

// Instance register or returns named application
func Instance(name ...string) *Application {
	appname := defaultAppName
	if len(name) > 0 {
		appname = name[0]
	}
	if appInstances[appname] == nil {
		appInstances[appname] = NewApplication(appname)
	}
	return appInstances[appname]
}

// Default initial a default app then returns
func Default() *Application {
	return Instance(defaultAppName)
}

// set session  当provider为nil时，使用默认的内存session
func (b *Application) SetSession(sessioopen bool, provider IProvider, option ...SessionOptions) {
	b.sessioopen = sessioopen
	if sessioopen {
		if provider != nil {
			b.provider = provider
			if len(option) > 0 {
				b.provider.Init(option[0])
			}
		} else { //默认内存session
			b.provider = NewMemProvider()
			if len(option) > 0 {
				b.provider.Init(option[0])
			} else {
				b.provider.Init(SessionOptions{Session_lifetime: 1800})
			}
		}
	}
}

// SetDIer set baa di
func (b *Application) SetDIer(v DIer) {
	b.di = v
}

// SetDebug set baa debug
func (b *Application) SetDebug(v bool) {
	b.debug = v
}

// Debug returns baa debug state
func (b *Application) Debug() bool {
	return b.debug
}

// Logger return baa logger
func (b *Application) Logger() Logger {
	return b.GetDI("logger").(Logger)
}

// Render return baa render
func (b *Application) Render() Renderer {
	return b.GetDI("render").(Renderer)
}

// Router return baa router
func (b *Application) Router() *Router {
	if b.router == nil {
		b.router = b.GetDI("router").(*Router)
	}
	return b.router
}

// SetDI registers a dependency injection
func (b *Application) SetDI(name string, h interface{}) {
	switch name {
	case "logger":
		if _, ok := h.(Logger); !ok {
			panic("DI logger must be implement interface baa.Logger")
		}
	case "render":
		if _, ok := h.(Renderer); !ok {
			panic("DI render must be implement interface baa.Renderer")
		}
	case "router":
		if _, ok := h.(IRouter); !ok {
			panic("DI router must be implement interface baa.Router")
		}
	}
	b.di.Set(name, h)
}

// GetDI fetch a registered dependency injection
func (b *Application) GetDI(name string) interface{} {
	return b.di.Get(name)
}

// Websocket register a websocket router handler
func (b *Application) Websocket(pattern string, wsHandler func(*websocket.Conn, *Context)) {
	b.Router().Get(pattern, func(c *Context) {
		conn, err := upgrader.Upgrade(c.Resp, c.Req, nil)
		if err != nil {
			b.Logger().Printf("websocket upgrade connection error: %v", err)
			return
		}
		defer conn.Close()
		wsHandler(conn, c)
	})
}

// SetNotFound set not found route handler
func (b *Application) SetNotFound(h HandlerFunc) {
	b.notFoundHandler = h
}

// NotFound execute not found handler
func (b *Application) NotFound(c *Context) {
	if b.notFoundHandler != nil {
		b.notFoundHandler(c)
		return
	}
	http.NotFound(c.Resp, c.Req)
}

// SetError set error handler
func (b *Application) SetError(h ErrorHandleFunc) {
	b.errorHandler = h
}

// Error execute internal error handler
func (b *Application) Error(err error, c *Context) {
	if err == nil {
		err = errors.New("Internal Server Error")
	}
	if b.errorHandler != nil {
		b.errorHandler(err, c)
		return
	}
	code := http.StatusInternalServerError
	msg := http.StatusText(code)
	if b.debug {
		msg = err.Error()
	}
	b.Logger().Println(err)
	http.Error(c.Resp, msg, code)
}

// DefaultNotFoundHandler invokes the default HTTP error handler.
func (b *Application) DefaultNotFoundHandler(c *Context) {
	code := http.StatusNotFound
	msg := http.StatusText(code)
	http.Error(c.Resp, msg, code)
}

func init() {
	appInstances = make(map[string]*Application)
	Env = os.Getenv("GOBLACK_ENV")
	if Env == "" {
		Env = DEV
	}
}

// Server returns the internal *http.Server.
func (b *Application) Server(addr string) *http.Server {
	s := &http.Server{Addr: addr}
	return s
}

// Run runs a server.
func (b *Application) Run(addr string) {
	b.run(b.Server(addr))
}

// RunTLS runs a server with TLS configuration.
func (b *Application) RunTLS(addr, certfile, keyfile string) {
	b.run(b.Server(addr), certfile, keyfile)
}

// RunServer runs a custom server.
func (b *Application) RunServer(s *http.Server) {
	b.run(s)
}

// RunTLSServer runs a custom server with TLS configuration.
func (b *Application) RunTLSServer(s *http.Server, crtFile, keyFile string) {
	b.run(s, crtFile, keyFile)
}

func (b *Application) run(s *http.Server, files ...string) {
	s.Handler = b
	b.Logger().Printf("Run mode: %s", Env)
	if len(files) == 0 {
		b.Logger().Printf("Listen %s", s.Addr)
		b.Logger().Fatal(s.ListenAndServe())
	} else if len(files) == 2 {
		b.Logger().Printf("Listen %s with TLS", s.Addr)
		b.Logger().Fatal(s.ListenAndServeTLS(files[0], files[1]))
	} else {
		panic("invalid TLS configuration")
	}
}

func (b *Application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := b.pool.Get().(*Context)
	c.Reset(w, r)
	if b.sessioopen {
		if len(c.GetHeader("x-token")) > 0 {
			var err error
			c.sessionStore, err = b.provider.Start(c)
			if err != nil {
				//b.Logger().Printf("session start err: %s", err.Error())
			}
		}
	}
	// build handler chain
	path := strings.Replace(r.URL.Path, "//", "/", -1)
	controlerstr, hooks, handlers, founded := b.Router().Match(r.Method, path, c)
	//c.routeName = name

	if founded {
		if len(hooks) > 0 {
			c.handlers = append(c.handlers, hooks...)
		}
		if len(handlers) > 0 {
			c.handlers = append(c.handlers, handlers...)
		} else if controlerstr != "" {
			h, ok := b.Router().GetController(controlerstr)
			if ok {
				c.handlers = append(c.handlers, h)
			} else {
				c.handlers = append(c.handlers, b.notFoundHandler)
			}
		}
	} else {
		c.handlers = append(c.handlers, b.notFoundHandler)
	}

	c.Next()
	if b.sessioopen {
		if c.sessionStore != nil {
			err := c.sessionStore.Save()
			if err != nil {
				b.Logger().Printf("session save err: %s", err.Error())
			}
		}
	}
	b.pool.Put(c)
}
