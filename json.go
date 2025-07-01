//go:build !jsoniter
// +build !jsoniter

package goblack

import "encoding/json"

var (
	Marshal       = json.Marshal
	Unmarshal     = json.Unmarshal
	MarshalIndent = json.MarshalIndent
)
