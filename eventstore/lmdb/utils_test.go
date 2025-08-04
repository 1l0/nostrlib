package lmdb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuickselect(t *testing.T) {
	its := iterators{
		{last: 781},
		{last: 900},
		{last: 1},
		{last: 81},
		{last: 325},
		{last: 781},
		{last: 562},
		{last: 81},
		{last: 444},
	}

	its.quickselect(3, 0, len(its))
	require.ElementsMatch(t, its[len(its)-3:], iterators{{last: 900}, {last: 781}, {last: 781}})

	its.quickselect(4, 0, len(its))
	require.ElementsMatch(t, its[len(its)-4:], iterators{{last: 562}, {last: 900}, {last: 781}, {last: 781}})
}
