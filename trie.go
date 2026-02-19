// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package goblack

import (
	"strings"
)

var Router_SplitChar = "/"

// 比较两数大小，返回小的
func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

// 最大公共前缀。注意不是字符串，是前缀 比较a，b字符串同文字有几个相同字符
func longestCommonPrefix(a, b string) int {
	i := 0
	max := min(len(a), len(b))
	for i < max && a[i] == b[i] {
		i++
	}
	return i
}

// 找出通配符:*的字符串和文字  Search for a wildcard segment and check the name for invalid characters.
// Returns -1 as index, if no wildcard was found.  再 两个 / ,/ 之间只能有一个通配符
// wilcard通配符，包含:，*  i 开始位置   valid是否有效
func findWildcard(path string) (wilcard string, i int, valid bool) {
	// Find start
	for start, c := range []byte(path) {
		// A wildcard starts with ':' (param) or '*' (catch-all)
		if c != ':' && c != '*' {
			continue
		}

		// Find end and check for invalid characters
		valid = true
		for end, c := range []byte(path[start+1:]) {
			switch c {
			case Router_SplitChar[0]: //分隔符
				return path[start : start+1+end], start, valid
			case ':', '*':
				valid = false
			}
		}
		return path[start:], start, valid
	}
	return "", -1, false
}

func countParams(path string) uint16 {
	var n uint
	for i := range []byte(path) {
		switch path[i] {
		case ':', '*':
			n++
		}
	}
	return uint16(n)
}

type nodeType uint8

const (
	staticpath nodeType = iota // default
	root
	param
	catchAll
)

type node struct {
	//当前node 的路径
	path string
	// children对应的索引，保存的是分裂分支的第一个字符，下级节点的第一个字符
	//子节点索引，当子节点为非参数类型，即本节点的 wildChild 为 false 时，会将每个子节点的首字母放在该索引数组。说是数组，实际上是个 string
	indices string
	//判断子节点是否为通配节点，（包括：param、catchAll）
	wildChild bool
	//节点类型。
	//root： 第一个插入的节点
	//catchAll: 有* 匹配的节点
	//param：参数节点 :id
	//staticpath： 静态节点，前几个剩下的都是静态节点
	nType nodeType
	// 优先级
	priority uint32 // 优先级
	//子节点
	children []*node //子节点
	//当前节点的处理函数
	//handle Handle //当前节点的处理函数
	//对应的action  记得替换:id 参数
	controller string //对应的action  记得替换:id 参数
	// 新增：直接注册的处理函数
	handlers []HandlerFunc // 新增：直接注册的处理函数
	// hook的string  @clearAll
	hook []HandlerFunc // hook的string  @clearAll
}

// Increments priority of the given child and reorders if necessary
// 寻找传入的路由与节点的最长公共前缀
func (n *node) incrementChildPrio(pos int) int {
	cs := n.children
	cs[pos].priority++
	prio := cs[pos].priority

	// Adjust position (move to front)
	newPos := pos
	for ; newPos > 0 && cs[newPos-1].priority < prio; newPos-- {
		// Swap node positions
		cs[newPos-1], cs[newPos] = cs[newPos], cs[newPos-1]
	}

	// Build new index char string
	if newPos != pos {
		n.indices = n.indices[:newPos] + // Unchanged prefix, might be empty
			n.indices[pos:pos+1] + // The index char we move
			n.indices[newPos:pos] + n.indices[pos+1:] // Rest without char at 'pos'
	}

	return newPos
}

// addRoute adds a node with the given handle to the path.
// Not concurrency-safe!
func (n *node) addRoute(path string, controller string, handlers []HandlerFunc, hook []HandlerFunc) {
	fullPath := path
	n.priority++

	// Empty tree
	if n.path == "" && n.indices == "" { //插入节点作为root节点
		n.insertChild(path, fullPath, controller, handlers, hook)
		n.nType = root
		return
	}

walk:
	for {
		// Find the longest common prefix.
		// This also implies that the common prefix contains no ':' or '*'
		// since the existing key can't contain those chars.
		// 获取到 word 和当前节点 path 的公共前缀长度
		i := longestCommonPrefix(path, n.path) //最长公共前缀

		// Split edge
		// Split edge--分割 边际
		// 如果最长前缀的长度 < 节点长度
		// 公共前缀小于当前节点的相对路径，需要对节点进行分解
		if i < len(n.path) {
			//那么当前节点的path就应该是最长前缀，原先的节点是最长前缀的子节点
			child := node{
				//节点的路径就是 当前节点后段字符串
				path:       n.path[i:],
				wildChild:  n.wildChild,
				nType:      staticpath,
				indices:    n.indices,
				children:   n.children,
				controller: n.controller,
				handlers:   n.handlers,
				hook:       n.hook,
				priority:   n.priority - 1,
			}
			//刚分裂的节点，作为新节点的子节点
			n.children = []*node{&child}
			// []byte for proper unicode char conversion, see #65
			n.indices = string([]byte{n.path[i]})
			n.path = path[:i]
			n.wildChild = false
		}
		// Make new node a child of this node    ----为当前节点创建一个新的子节点
		//如果最长前缀 < 路径的长度
		//除了tree为空的情况外，只有在这个判断下才会走 insertChild ，创建新节点
		// Make new node a child of this node
		if i < len(path) {
			// 对 path 扣除公共前缀部分
			path = path[i:]
			//如果子节点是参数节点
			if n.wildChild {
				n = n.children[0]
				n.priority++
				// Check if the wildcard matches
				// 如果子节点是通配节点，需要验证合法性
				// Check if the wildcard matches
				if len(path) >= len(n.path) && n.path == path[:len(n.path)] &&
					// Adding a child to a catchAll is not possible
					n.nType != catchAll &&
					// Check for longer wildcard, e.g. :name and :names
					(len(n.path) >= len(path) || path[len(n.path)] == Router_SplitChar[0]) {
					continue walk
				} else {
					// Wildcard conflict
					// Wildcard conflict
					// 通配符冲突
					pathSeg := path
					if n.nType != catchAll {
						// SplitN 根据/分割字符串，最多2个
						pathSeg = strings.SplitN(pathSeg, Router_SplitChar, 2)[0]
					}
					prefix := fullPath[:strings.Index(fullPath, pathSeg)] + n.path
					panic("'" + pathSeg +
						"' in new path '" + fullPath +
						"' conflicts with existing wildcard '" + n.path +
						"' in existing prefix '" + prefix +
						"'")
				}
			}

			idxc := path[0]

			// '/' after param
			if n.nType == param && idxc == Router_SplitChar[0] && len(n.children) == 1 {
				n = n.children[0]
				n.priority++
				continue walk
			}
			// Check if a child with the next path byte exists
			//判断当前节点的 indices 索引中是否存在当前path的首字母
			// Check if a child with the next path byte exists
			for i, c := range []byte(n.indices) {
				if c == idxc {
					i = n.incrementChildPrio(i)
					n = n.children[i]
					continue walk
				}
			}

			// Otherwise insert it  ---否则直接插入
			if idxc != ':' && idxc != '*' {
				// []byte for proper unicode char conversion, see #65
				n.indices += string([]byte{idxc})
				child := &node{}
				n.children = append(n.children, child)
				n.incrementChildPrio(len(n.indices) - 1)
				n = child
			}
			n.insertChild(path, fullPath, controller, handlers, hook)
			return
		}

		// Otherwise add handle to current node
		//如果最长前缀 == 传入的path，就直接在这个节点添加handle
		if n.controller != "" || len(n.handlers) > 0 {
			panic("a handle is already registered for path '" + fullPath + "'")
		}
		n.controller = controller
		n.handlers = handlers
		n.hook = hook
		return
	}
}

func (n *node) insertChild(path, fullPath string, controller string, handlers []HandlerFunc, hook []HandlerFunc) {
	for {
		// Find prefix until first wildcard
		wildcard, i, valid := findWildcard(path)
		if i < 0 { // No wilcard found
			break
		}

		// The wildcard name must not contain ':' and '*'
		if !valid {
			panic("only one wildcard per path segment is allowed, has: '" +
				wildcard + "' in path '" + fullPath + "'")
		}

		// Check if the wildcard has a name
		if len(wildcard) < 2 {
			panic("wildcards must be named with a non-empty name in path '" + fullPath + "'")
		}

		// Check if this node has existing children which would be
		// unreachable if we insert the wildcard here 插入的肯定是新节点，不能有子节点
		if len(n.children) > 0 {
			panic("wildcard segment '" + wildcard +
				"' conflicts with existing children in path '" + fullPath + "'")
		}

		// param
		if wildcard[0] == ':' {
			if i > 0 {
				// Insert prefix before the current wildcard
				n.path = path[:i]
				path = path[i:]
			}

			n.wildChild = true
			child := &node{
				nType: param,
				path:  wildcard,
			}
			n.children = []*node{child}
			n = child
			n.priority++

			// If the path doesn't end with the wildcard, then there
			// will be another non-wildcard subpath starting with '/'
			if len(wildcard) < len(path) {
				path = path[len(wildcard):]
				child := &node{
					priority: 1,
				}
				n.children = []*node{child}
				n = child
				continue
			}

			// Otherwise we're done. Insert the handle in the new leaf
			n.controller = controller
			n.handlers = handlers
			n.hook = hook
			return
		}

		// catchAll  *
		if i+len(wildcard) != len(path) { //*只能再最后
			panic("catch-all routes are only allowed at the end of the path in path '" + fullPath + "'")
		}

		if len(n.path) > 0 && n.path[len(n.path)-1] == Router_SplitChar[0] {
			panic("catch-all conflicts with existing handle for the path segment root in path '" + fullPath + "'")
		}

		// Currently fixed width 1 for '/'  *前面不能是/
		i--
		if path[i] != Router_SplitChar[0] {
			panic("no / before catch-all in path '" + fullPath + "'")
		}

		n.path = path[:i]

		// First node: catchAll node with empty path
		child := &node{
			wildChild: true,
			nType:     catchAll,
		}
		n.children = []*node{child}
		n.indices = Router_SplitChar
		n = child
		n.priority++

		// Second node: node holding the variable
		child = &node{
			path:       path[i:],
			nType:      catchAll,
			controller: n.controller,
			handlers:   n.handlers,
			hook:       n.hook,
			priority:   1,
		}
		n.children = []*node{child}

		return
	}

	// If no wildcard was found, simply insert the path and handle
	n.path = path
	n.controller = controller
	n.handlers = handlers
	n.hook = hook
}

/*
// Returns the handle registered with the given path (key). The values of
// wildcards are saved to a map.
// If no handle can be found, a TSR (trailing slash redirect) recommendation is
// made if a handle exists with an extra (without the) trailing slash for the
// given path.  /blog/:post/
func (n *node) getValue(path string, params func() *Params) (handle Handle, ps *Params, tsr bool) {
walk: // Outer loop for walking the tree
	for {
		prefix := n.path //blog
		if len(path) > len(prefix) {
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):] //  /:post/

				// If this node does not have a wildcard (param or catchAll)
				// child, we can just look up the next child node and continue
				// to walk down the tree
				if !n.wildChild { //如果下级节点不是变参，则判断path第一字符是不是再下级第一字符索引中，再则循环，不在则返回
					idxc := path[0]
					for i, c := range []byte(n.indices) {
						if c == idxc {
							n = n.children[i]
							continue walk
						}
					}

					// Nothing found.
					// We can recommend to redirect to the same URL without a
					// trailing slash if a leaf exists for that path.
					tsr = (path == "/" && n.handle != nil) //路径只有 / 并且 ccontrol不为空，则 tsr =true
					return
				}
				//下级子节点是变参
				// Handle wildcard child
				n = n.children[0]
				switch n.nType {
				case param:
					// Find param end (either '/' or path end)
					end := 0
					for end < len(path) && path[end] != '/' {
						end++
					}
					//保存路径为参数
					// Save param value
					if params != nil {
						if ps == nil {
							ps = params()
						}
						// Expand slice within preallocated capacity  在预先分配的容量内扩展切片
						i := len(*ps)
						*ps = (*ps)[:i+1]
						(*ps)[i] = Param{ //把key value 添加到 ps
							Key:   n.path[1:],
							Value: path[:end],
						}
					}

					// We need to go deeper! //变参下面还有没处理的，却n存在下级子节点，则举行循环，没有则tsr=true
					if end < len(path) {
						if len(n.children) > 0 {
							path = path[end:]
							n = n.children[0]
							continue walk
						}

						// ... but we can't
						tsr = (len(path) == end+1)
						return
					}

					if handle = n.handle; handle != nil { //当前节点有handle，则返回，没有则判断下一节点有没handle。。。，返回tsr
						return
					} else if len(n.children) == 1 {
						// No handle found. Check if a handle for this path + a
						// trailing slash exists for TSR recommendation
						n = n.children[0]
						tsr = (n.path == "/" && n.handle != nil) || (n.path == "" && n.indices == "/")
					}

					return
					//*
				case catchAll:
					// Save param value
					if params != nil {
						if ps == nil {
							ps = params()
						}
						// Expand slice within preallocated capacity
						i := len(*ps)
						*ps = (*ps)[:i+1]
						(*ps)[i] = Param{
							Key:   n.path[2:],
							Value: path,
						}
					}

					handle = n.handle
					return

				default:
					panic("invalid node type")
				}
			}
		} else if path == prefix {
			// We should have reached the node containing the handle.
			// Check if this node has a handle registered.
			if handle = n.handle; handle != nil {
				return
			}

			// If there is no handle for this route, but this route has a
			// wildcard child, there must be a handle for this path with an
			// additional trailing slash
			if path == "/" && n.wildChild && n.nType != root {
				tsr = true
				return
			}

			if path == "/" && n.nType == staticpath {
				tsr = true
				return
			}

			// No handle found. Check if a handle for this path + a
			// trailing slash exists for trailing slash recommendation
			for i, c := range []byte(n.indices) {
				if c == '/' {
					n = n.children[i]
					tsr = (len(n.path) == 1 && n.handle != nil) ||
						(n.nType == catchAll && n.children[0].handle != nil)
					return
				}
			}
			return
		}

		// Nothing found. We can recommend to redirect to the same URL with an
		// extra trailing slash if a leaf exists for that path
		tsr = (path == "/") ||
			(len(prefix) == len(path)+1 && prefix[len(path)] == '/' &&
				path == prefix[:len(prefix)-1] && n.handle != nil)
		return
	}
}
*/

/*
controler string
说明：匹配到的路由节点所绑定的 controller（控制器/处理函数名）。
用途：你可以通过它找到应该处理该请求的业务逻辑。
hook []HandlerFunc
说明：匹配到的路由节点所绑定的钩子函数（中间件/前置处理器等）。
用途：在执行 controller 之前或之后，可以执行这些 hook 做一些额外处理，比如鉴权、日志等。
vars map[string]string
说明：路径参数的键值对映射。例如路由 /user/:id 匹配 /user/123 时，vars 就会是 {"id": "123"}。
用途：用于获取 URL 路径中的参数，方便业务逻辑使用。
founded bool
说明：是否成功匹配到一个有效的路由节点。
用途：如果为 true，说明找到了对应的 controller 和 hook；如果为 false，说明没有匹配到。
*/
func (n *node) getValue(path string) (controller string, handlers []HandlerFunc, hook []HandlerFunc, vars map[string]string, founded bool) {
	founded = false
walk: // Outer loop for walking the tree
	for {
		prefix := n.path //blog
		if len(path) > len(prefix) {
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):] //  /:post/

				// If this node does not have a wildcard (param or catchAll)
				// child, we can just look up the next child node and continue
				// to walk down the tree
				if !n.wildChild { //如果下级节点不是变参，则判断path第一字符是不是再下级第一字符索引中，再则循环，不在则返回
					idxc := path[0]
					for i, c := range []byte(n.indices) {
						if c == idxc {
							n = n.children[i]
							continue walk
						}
					}

					// Nothing found.
					// We can recommend to redirect to the same URL without a
					// trailing slash if a leaf exists for that path.
					//tsr = (path == "/" && n.handle != nil) //路径只有 / 并且 ccontrol不为空，则 tsr =true
					founded = false
					return
				}
				//下级子节点是变参
				// Handle wildcard child
				n = n.children[0]
				switch n.nType {
				case param: // :id
					// Find param end (either '/' or path end)
					end := 0
					for end < len(path) && path[end] != Router_SplitChar[0] {
						end++
					}

					if vars == nil {
						vars = make(map[string]string, 2)
					}
					vars[n.path[1:]] = path[:end]
					//保存路径为参数
					// Save param value
					/*	if params != nil {
							if ps == nil {
								ps = params()
							}
							// Expand slice within preallocated capacity  在预先分配的容量内扩展切片
							i := len(*ps)
							*ps = (*ps)[:i+1]
							(*ps)[i] = Param{ //把key value 添加到 ps
								Key:   n.path[1:],
								Value: path[:end],
							}
						}
					*/
					// We need to go deeper! //变参下面还有没处理的，却存在下级子节点，则举行循环，没有则tsr=true
					if end < len(path) {
						if len(n.children) > 0 {
							path = path[end:]
							n = n.children[0]
							continue walk
						}

						// ... but we can't
						//tsr = (len(path) == end+1)
						founded = false
						return
					}

					if len(n.handlers) > 0 {
						handlers = n.handlers
						hook = n.hook
						founded = true
						return
					}
					if n.controller != "" {
						controller = n.controller
						hook = n.hook
						founded = true
						return
					}
					if len(n.children) == 1 {
						n = n.children[0]
						founded = false
					}
					return
					//*
				case catchAll:
					// Save param value
					if vars == nil {
						vars = make(map[string]string, 2)
					}
					vars[n.path[2:]] = path
					/*if params != nil {
						if ps == nil {
							ps = params()
						}
						// Expand slice within preallocated capacity
						i := len(*ps)
						*ps = (*ps)[:i+1]
						(*ps)[i] = Param{
							Key:   n.path[2:],
							Value: path,
						}
					}*/

					if len(n.handlers) > 0 {
						handlers = n.handlers
						hook = n.hook
						founded = true
						return
					}
					if n.controller != "" {
						controller = n.controller
						hook = n.hook
						founded = true
						return
					}
				}
			}
		} else if path == prefix {
			// We should have reached the node containing the handle.
			// Check if this node has a handle registered.
			if len(n.handlers) > 0 {
				handlers = n.handlers
				hook = n.hook
				founded = true
				return
			}
			if n.controller != "" {
				controller = n.controller
				hook = n.hook
				founded = true
				return
			}
			founded = false
			return
		}

		// Nothing found. We can recommend to redirect to the same URL with an
		// extra trailing slash if a leaf exists for that path
		founded = false
		//	tsr = (path == "/") ||
		//	(len(prefix) == len(path)+1 && prefix[len(path)] == '/' &&
		//		path == prefix[:len(prefix)-1] && n.handle != nil)
		return
	}
}
