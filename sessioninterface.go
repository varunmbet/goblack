package goblack

import (
	"bytes"
	"encoding/gob"
)

type SessionOptions struct {
	Session_lifetime int64
	GCInterval       int64 //多长时间检查一次过期
	IDLength         int
	Cookieoption     CookieOptions
	Providerconfig   interface{}
}

// CookieOptions session manager cookie options
type CookieOptions struct {
	Cookie_key string
	Domain     string
	Path       string
	Secure     bool
	LifeTime   int64
	HttpOnly   bool
}

// Provider sessio provider interface
type IProvider interface {

	// Destroy destroy session id from provider
	//Destroy(sid string) error
	// GC calls GC to clean expired sessions
	//GC()

	Init(options SessionOptions) error
	Start(c *Context, withsid ...string) (*Session, error)
	// Exist check session id is exist
	Exist(sid string) bool
	Load(sess_id string) (*Session, error)
	Save(sid string, data map[interface{}]interface{}) error
}

func init() {
	gob.Register([]interface{}{})
	gob.Register(map[int]interface{}{})
	gob.Register(map[string]interface{}{})
	gob.Register(map[interface{}]interface{}{})
	gob.Register(map[string]string{})
	gob.Register(map[int]string{})
	gob.Register(map[int]int{})
	gob.Register(map[int]int64{})
}

func NewCookieOption(config CookieOptions) CookieOptions {
	var cookieOption CookieOptions
	if config.Cookie_key != "" {
		cookieOption.Cookie_key = config.Cookie_key
	} else {
		cookieOption.Cookie_key = "SESSION"
	}
	if config.Domain != "" {
		cookieOption.Domain = config.Domain
	}
	if config.LifeTime > 0 {
		cookieOption.LifeTime = config.LifeTime
	} else {
		cookieOption.LifeTime = 7776000
	}
	if config.Path != "" {
		cookieOption.Path = config.Path
	} else {
		cookieOption.Path = "/"
	}
	cookieOption.Secure = config.Secure
	cookieOption.HttpOnly = config.HttpOnly
	return cookieOption
}

// EncodeGob encode data by gob
func EncodeGob(object map[interface{}]interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(object)
	if err == nil {
		return buf.Bytes(), nil
	}
	return nil, err
}

// DecodeGob decode data by gob
func DecodeGob(encoded []byte) (out map[interface{}]interface{}, err error) {
	buf := bytes.NewBuffer(encoded)
	err = gob.NewDecoder(buf).Decode(&out)
	return out, err
}
