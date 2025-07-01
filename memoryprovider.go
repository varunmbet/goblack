package goblack

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// MemoryProvider memory session provider
type MemoryProvider struct {
	IProvider
	lock         sync.RWMutex
	bytes        int64
	data         map[string]*list.Element
	list         *list.List
	options      MemoryOptions
	cookieOption CookieOptions
	maxLifeTime  int64 //过期时长
	idLength     int
	GCInterval   int64 //多长时间检查一次过期
}

// MemoryOptions memory session provider options
type MemoryOptions struct {
	BytesLimit int64
}

type item struct {
	sid        string
	lastAccess time.Time
	data       []byte
}

// Init initialize provider
func (p *MemoryProvider) Init(options SessionOptions) error {
	if options != (SessionOptions{}) {
		p.cookieOption = NewCookieOption(options.Cookieoption)
		if options.Session_lifetime > 0 {
			p.maxLifeTime = options.Session_lifetime
		} else {
			p.maxLifeTime = 60 * 30
		}
		if options.Providerconfig != nil {
			p.options = options.Providerconfig.(MemoryOptions)
		}
		if options.IDLength > 0 {
			p.idLength = options.IDLength
		} else {
			p.idLength = 16
		}

		if options.GCInterval == 0 {
			if options.Session_lifetime > 0 {
				p.GCInterval = options.Session_lifetime
			} else {
				p.GCInterval = 1440
			}
		} else {
			p.GCInterval = options.GCInterval
		}
	}
	p.startGC()
	return nil
}

// Start begin session manager
func (p *MemoryProvider) Start(c *Context, withsid ...string) (*Session, error) {
	var session *Session

	sid := c.GetCookie(p.cookieOption.Cookie_key)
	if len(sid) > 0 && p.Exist(sid) {
		return p.Load(sid)
	}
	if len(withsid) > 0 {
		sid = withsid[0]
	} else {
		sid = CreateSessionID(p.idLength)
	}
	session, err := p.Load(sid)
	if err != nil {
		return nil, err
	}

	c.SetCookie(
		p.cookieOption.Cookie_key,
		sid,
		p.cookieOption.LifeTime,
		p.cookieOption.Path,
		p.cookieOption.Domain,
		p.cookieOption.Secure,
		p.cookieOption.HttpOnly,
	)

	return session, nil
}

// Exist check session id is exist
func (p *MemoryProvider) Exist(sid string) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	_, ok := p.data[sid]
	return ok
}

// Read read session data from provider
func (p *MemoryProvider) Load(sid string) (*Session, error) {
	p.lock.RLock()
	elem, ok := p.data[sid]
	p.lock.RUnlock()

	if !ok {
		return NewSession(p, sid, nil)
	}

	p.touch(sid)

	data, err := DecodeGob(elem.Value.(*item).data)
	if err != nil {
		return nil, err
	}

	return NewSession(p, sid, data)
}

func (p *MemoryProvider) touch(sid string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if elem, ok := p.data[sid]; ok {
		elem.Value.(*item).lastAccess = time.Now()
		p.list.MoveToFront(elem)
	}

	return nil
}

// Write write session data to provider
func (p *MemoryProvider) Save(sid string, data map[interface{}]interface{}) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	encoded, err := EncodeGob(data)
	if err != nil {
		return err
	}

	bytes := p.bytes + int64(len(encoded))
	if bytes > p.options.BytesLimit {
		return fmt.Errorf("session size reached to size limit %d", p.options.BytesLimit)
	}

	p.bytes = bytes
	p.data[sid] = p.list.PushBack(&item{
		sid:        sid,
		lastAccess: time.Now(),
		data:       encoded,
	})

	return nil
}

// Destroy destroy session id from provider
func (p *MemoryProvider) Destroy(sid string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if elem, ok := p.data[sid]; ok {
		p.bytes -= int64(len(elem.Value.(*item).data))
		p.list.Remove(elem)
		delete(p.data, sid)
	}

	return nil
}

func (p *MemoryProvider) startGC() {
	p.GC()
	time.AfterFunc(time.Duration(p.GCInterval)*time.Second, func() {
		p.startGC()
	})
}

// GC calls GC to clean expired sessions
func (p *MemoryProvider) GC() {
	p.lock.RLock()

	for {
		elem := p.list.Back()
		if elem == nil {
			break
		}

		if elem.Value.(*item).lastAccess.Unix()+p.maxLifeTime < time.Now().Unix() {
			p.lock.RUnlock()
			p.lock.Lock()
			p.bytes -= int64(len(elem.Value.(*item).data))
			p.list.Remove(elem)
			delete(p.data, elem.Value.(*item).sid)
			p.lock.Unlock()
			p.lock.RLock()
		} else {
			break
		}
	}

	p.lock.RUnlock()
}

func NewMemProvider() *MemoryProvider {
	provider := MemoryProvider{}
	provider.list = list.New()
	provider.data = make(map[string]*list.Element)
	provider.maxLifeTime = 1800
	provider.GCInterval = 1200
	provider.options = MemoryOptions{BytesLimit: 1024 * 60}
	return &provider
	/*return &MemoryProvider{
		list:        list.New(),
		data:        make(map[string]*list.Element),
		maxLifeTime: 1800,
		GCInterval:  1400,
	}*/
}
