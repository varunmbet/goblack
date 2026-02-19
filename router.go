package goblack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

const (
	MethodGet     = "GET"
	MethodHead    = "HEAD"
	MethodPost    = "POST"
	MethodPut     = "PUT"
	MethodPatch   = "PATCH"
	MethodDelete  = "DELETE"
	MethodConnect = "CONNECT"
	MethodOptions = "OPTIONS"
	MethodTrace   = "TRACE"
)

// Router is an router interface for baa
type IRouter interface {
	// SetAutoHead sets the value who determines whether add HEAD method automatically
	// when GET method is added. Combo router will not be affected by this value.
	SetAutoHead(v bool)
	// SetAutoTrailingSlash optional trailing slash.
	SetAutoTrailingSlash(v bool)
	// Match find matched route then returns handlers and name
	Match(method, urlPath string, c *Context) (controller string, hook []HandlerFunc, handlers []HandlerFunc, founded bool)
	// URLFor use named route return format url
	//URLFor(name string, args ...interface{}) string
	// Add registers a new handle with the given method, pattern and handlers.
	//Add(method, pattern string, handlers []HandlerFunc) RouteNode
	// GroupAdd registers a list of same prefix route
	//GroupAdd(pattern string, f func(), handlers []HandlerFunc)
	// Routes returns registered route uri in a string slice
	//Routes() map[string][]string
	// NamedRoutes returns named route uri in a string slice
	//NamedRoutes() map[string]string
}

type Router struct {
	Trees             map[string]*node
	Hookfunc          []HandlerFunc   //当前分组的hook
	GroupString       string          //当前分组的path
	grouphookfunc     [][]HandlerFunc // 分组hook栈
	groupPrefix       []string        // 分组前缀栈
	Middleware        []HandlerFunc
	Controllers       map[string]HandlerFunc
	AutoHead          bool
	AutoTrailingSlash bool
}

// 初始化对象
//
// Initialize object
func NewRouter() IRouter {
	p := new(Router) //&Router{}
	//methods := []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE"}
	p.Trees = make(map[string]*node)
	p.Hookfunc = make([]HandlerFunc, 0)
	p.Middleware = make([]HandlerFunc, 0)
	p.Controllers = make(map[string]HandlerFunc)
	p.AutoHead = false
	p.AutoTrailingSlash = false
	/*	for _, method := range methods {
		tree := newTree()
		tree.name = string(method)
		p.treeGroup[method] = tree
	}*/
	return p
}

// SetAutoHead sets the value who determines whether add HEAD method automatically
// when GET method is added. Combo router will not be affected by this value.
func (r *Router) SetAutoHead(v bool) {
	r.AutoHead = v
}

// SetAutoTrailingSlash optional trailing slash.
func (r *Router) SetAutoTrailingSlash(v bool) {
	r.AutoTrailingSlash = v
}

// set Middleware
func (r *Router) Use(m ...Middleware) {
	for i := range m {
		if m[i] != nil {
			r.Middleware = append(r.Middleware, wrapMiddleware(m[i]))
		}
	}
}

// wrapMiddleware wraps middleware.
func wrapMiddleware(m Middleware) HandlerFunc {
	switch m := m.(type) {
	case HandlerFunc:
		return m
	case func(*Context):
		return m
	case http.Handler, http.HandlerFunc:
		return WrapHandlerFunc(func(c *Context) {
			m.(http.Handler).ServeHTTP(c.Resp, c.Req)
		})
	case func(http.ResponseWriter, *http.Request):
		return WrapHandlerFunc(func(c *Context) {
			m(c.Resp, c.Req)
		})
	default:
		panic("unknown middleware")
	}
}

// WrapHandlerFunc wrap for context handler chain
func WrapHandlerFunc(h HandlerFunc) HandlerFunc {
	return func(c *Context) {
		h(c)
		c.Next()
	}
}

func (r *Router) Head(urlRule string, args ...interface{}) {
	r.handleRegister("HEAD", urlRule, args...)
}

func (r *Router) Get(urlRule string, args ...interface{}) {
	r.handleRegister("GET", urlRule, args...)
	if r.AutoHead {
		r.handleRegister("HEAD", urlRule, args...)
	}
}

func (r *Router) Post(urlRule string, args ...interface{}) {
	r.handleRegister("POST", urlRule, args...)
}

func (r *Router) Put(urlRule string, args ...interface{}) {
	r.handleRegister("PUT", urlRule, args...)
}

func (r *Router) Patch(urlRule string, args ...interface{}) {
	r.handleRegister("PATCH", urlRule, args...)
}

func (r *Router) Delete(urlRule string, args ...interface{}) {
	r.handleRegister("DELETE", urlRule, args...)
}

func (r *Router) Connect(urlRule string, args ...interface{}) {
	r.handleRegister("CONNECT", urlRule, args...)
}

func (r *Router) Options(urlRule string, args ...interface{}) {
	r.handleRegister("OPTIONS", urlRule, args...)
}

func (r *Router) Tract(urlRule string, args ...interface{}) {
	r.handleRegister("TRACE", urlRule, args...)
}

// 新增统一注册方法，兼容 controller 字符串和 HandlerFunc
func (r *Router) handleRegister(method, urlRule string, args ...interface{}) {
	urlRule = strings.ToLower(urlRule)
	var controller string
	var handlers []HandlerFunc
	var hookfunc []HandlerFunc

	if r.GroupString != "" {
		urlRule = r.GroupString + urlRule
		for i := range r.Hookfunc {
			if r.Hookfunc[i] != nil {
				hookfunc = append(hookfunc, r.Hookfunc[i])
			}
		}
	}

	// 判断参数类型
	if len(args) > 0 {
		switch v := args[0].(type) {
		case string:
			controller = strings.ToLower(v)
			for i := 1; i < len(args); i++ {
				if mw, ok := args[i].(Middleware); ok && mw != nil {
					hookfunc = append(hookfunc, wrapMiddleware(mw))
				}
			}
		case HandlerFunc:
			for i := 0; i < len(args); i++ {
				if hf, ok := args[i].(HandlerFunc); ok && hf != nil {
					handlers = append(handlers, hf)
				}
			}
		case func(*Context): //匿名函数
			for i := 0; i < len(args); i++ {
				if hf, ok := args[i].(func(*Context)); ok && hf != nil {
					handlers = append(handlers, HandlerFunc(hf))
				}
			}
		}
	}

	if r.Trees == nil {
		r.Trees = make(map[string]*node)
	}
	root := r.Trees[method]
	if root == nil {
		root = new(node)
		r.Trees[method] = root
	}
	root.addRoute(urlRule, controller, handlers, hookfunc)
}

func (r *Router) Group(path string, m ...Middleware) {
	// push 当前状态
	r.grouphookfunc = append(r.grouphookfunc, r.Hookfunc)
	r.groupPrefix = append(r.groupPrefix, r.GroupString)

	// 新分组
	r.GroupString = r.GroupString + strings.TrimSpace(path)
	r.Hookfunc = append([]HandlerFunc{}, r.Hookfunc...)
	for i := range m {
		if m[i] != nil {
			r.Hookfunc = append(r.Hookfunc, wrapMiddleware(m[i]))
		}
	}
}

func (r *Router) EndGroup() {
	if len(r.grouphookfunc) > 0 {
		r.Hookfunc = r.grouphookfunc[len(r.grouphookfunc)-1]
		r.grouphookfunc = r.grouphookfunc[:len(r.grouphookfunc)-1]
	}
	if len(r.groupPrefix) > 0 {
		r.GroupString = r.groupPrefix[len(r.groupPrefix)-1]
		r.groupPrefix = r.groupPrefix[:len(r.groupPrefix)-1]
	}
	if len(r.groupPrefix) == 0 {
		r.GroupString = ""
	}
}

func (r *Router) Show() {
	for method, root := range r.Trees {
		showNode(method, root, "")
	}
}

func showNode(method string, n *node, prefix string) {
	if n == nil {
		return
	}
	path := prefix + n.path
	if n.controller != "" {
		fmt.Printf("[%s] %s => controller: %s\n", method, path, n.controller)
	}
	if len(n.handlers) > 0 {
		handlerInfo := "handler(s)"
		fmt.Printf("[%s] %s => %s\n", method, path, handlerInfo)
	}
	for _, child := range n.children {
		showNode(method, child, path)
	}
}

func (r *Router) String() string {
	root := r.Trees[MethodGet]
	if root == nil {
		return "{}"
	}
	b, _ := json.Marshal(*root)
	var out bytes.Buffer
	_ = json.Indent(&out, b, "", "  ")
	return out.String()
}

// 查询URL匹配的树节点并返回变量
//
// Query the tree node with matching URL and return variables
func (r *Router) Match(method, urlPath string, c *Context) (controller string, hook []HandlerFunc, handlers []HandlerFunc, founded bool) {
	urlPath = strings.ToLower(urlPath)
	if urlPath == "" {
		panic(fmt.Sprintf("web: path[%s] is not empty", urlPath))
	}
	if root := r.Trees[method]; root != nil {
		var vars map[string]string
		controller, handlers, hook, vars, founded = root.getValue(urlPath)
		for key, value := range vars {
			c.SetParam(key, value)
			controller = strings.Replace(controller, ":"+key, value, 1)
		}
		return controller, hook, handlers, founded
	}
	return "", nil, nil, false
}

func (r *Router) GetController(controlstr string) (controller HandlerFunc, founded bool) {
	if h, ok := r.Controllers[controlstr]; ok {
		return h, true
	} else {
		return nil, false
	}

}

// HandlerFunc 将控制器方法转为 gin.HandlerFunc 方法
func RouteToHandlerFunc(v Route) HandlerFunc {
	return func(c *Context) {
		arguments := make([]reflect.Value, 1)
		arguments[0] = reflect.ValueOf(c)
		v.Method.Call(arguments)
	}
}

// Route 数据格式为：
// Method:控制器方法对象
// Args:方法的参数
type Route struct {
	Method reflect.Value
	Args   []reflect.Type
}

// Routes 存储的数据格式为：Routes[控制器名称][方法名称]Route对象
// var Routes = make(map[string]map[string]Route)
//var Routes = make(map[string]HandlerFunc) //[控制器名称]@[方法名称]

// Register 注册路由方法 这个在controll中调用
/*
func init() {
	goblack.Instance("demo").Router().RegisterController(&Admin_admin{})
}
*/
func (r *Router) RegisterController(controller interface{}) bool {
	// 取得控制器的 reflect.Value，结果类似：&{}
	v := reflect.ValueOf(controller)
	// 取得控制器中导出的方法的数量，非控制器或无方法则直接返回，结果是个数字，比如：2
	if v.NumMethod() == 0 {
		return false
	}

	// 取得控制器的 reflect.Type，结果类似"*controller.Article"
	t := reflect.TypeOf(controller).String()
	// 设置控制器类名
	module := t
	if strings.Contains(t, ".") {
		// 此时结果类似"Article"
		module = t[strings.Index(t, ".")+1:]
	}

	// 遍历控制器的所有方法
	for i := 0; i < v.NumMethod(); i++ {
		// 获取第 i 个方法
		method := v.Method(i)

		// 获取第 i 个方法的名称
		action := v.Type().Method(i).Name

		// 遍历方法中传递过来的参数，保存到 params
		params := make([]reflect.Type, 0)
		for j := 0; j < method.Type().NumIn(); j++ {
			params = append(params, method.Type().In(j))
		}

		var controllerstr = strings.ToLower(module + "@" + action)
		r.Controllers[controllerstr] = RouteToHandlerFunc(Route{method, params})
		//if Routes[module] == nil {
		// 新控制类，则初始化
		//	Routes[module] = make(map[string]Route)
		//}
		//Routes[module][action] = Route{method, params}
	}
	return true
}
