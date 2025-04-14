package test

import (
	"testing"

	"fiatjaf.com/nostrlib/sdk/hints/memoryh"
)

func TestMemoryHints(t *testing.T) {
	runTestWith(t, memoryh.NewHintDB())
}
