package lmdb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuickselect(t *testing.T) {
	{
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

		its.quickselect(3)
		require.ElementsMatch(t,
			[]uint32{its[0].last, its[1].last, its[2].last},
			[]uint32{900, 781, 781},
		)
	}

	{
		its := iterators{
			{last: 781},
			{last: 781},
			{last: 900},
			{last: 1},
			{last: 87},
			{last: 315},
			{last: 789},
			{last: 500},
			{last: 812},
			{last: 306},
			{last: 612},
			{last: 444},
			{last: 59},
			{last: 441},
			{last: 901},
			{last: 901},
			{last: 2},
			{last: 81},
			{last: 325},
			{last: 781},
			{last: 562},
			{last: 81},
			{last: 326},
			{last: 662},
			{last: 444},
			{last: 81},
			{last: 444},
		}

		its.quickselect(6)
		require.ElementsMatch(t,
			[]uint32{its[0].last, its[1].last, its[2].last, its[3].last, its[4].last, its[5].last},
			[]uint32{901, 900, 901, 781, 812, 789},
		)
	}
}
