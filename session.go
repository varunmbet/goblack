package goblack

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Session session bag
type Session struct {
	//ISession
	provider IProvider
	sid      string
	lock     sync.RWMutex
	data     map[interface{}]interface{}
}

// ID returns session id
func (s *Session) SessionID() string {
	return s.sid
}

// Get get a value by the key from session
func (s *Session) Get(key ...interface{}) interface{} {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if len(key) > 0 {
		if v, ok := s.data[key[0]]; ok {
			return v
		}
		return nil
	} else {
		return s.data
	}
}

// Set set a value by the key to session
func (s *Session) Set(key interface{}, val interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data[key] = val
	return nil
}

func (s *Session) Has(key interface{}) bool {
	if _, ok := s.data[key]; ok {
		return true
	}
	return false
}

// Delete delete a key from session
func (s *Session) Delete(key interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.data, key)
	return nil
}

// Destroy remove all data stored in a session
func (s *Session) Clear() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.data = make(map[interface{}]interface{})
	return nil
}

// Close save data in session
func (s *Session) Save() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.provider.Save(s.sid, s.data)
}

// NewSession create a session
func NewSession(provider IProvider, sid string, data map[interface{}]interface{}) (*Session, error) {
	if provider == nil {
		return nil, fmt.Errorf("session.New(): provider cannot be nil")
	}

	if len(sid) == 0 {
		return nil, fmt.Errorf("session.New(): invalid session id")
	}

	if data == nil {
		data = make(map[interface{}]interface{})
	}

	return &Session{
		provider: provider,
		sid:      sid,
		data:     data,
	}, nil
}

func CreateSessionID(idlen int) string {
	return randString(idlen)
}

const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func randString(n int) string {
	var letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	src := rand.NewSource(time.Now().UnixNano())
	b := make([]byte, n)
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}
