//go:build !tinygo

package nostr

import "github.com/mailru/easyjson"

func (ef Filter) String() string {
	j, _ := easyjson.Marshal(ef)
	return string(j)
}
