//go:build !tinygo

package nip46

import jsoniter "github.com/json-iterator/go"

var json = jsoniter.ConfigFastest
