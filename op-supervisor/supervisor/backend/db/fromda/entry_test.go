package fromda

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func FuzzRoundtripLinkEntry(f *testing.F) {
	f.Fuzz(func(t *testing.T, aHash []byte, aNum uint64, aTimestamp uint64, bHash []byte, bNum uint64, bTimestamp uint64) {
		x := LinkEntry{
			source: types.BlockSeal{
				Hash:      common.BytesToHash(aHash),
				Number:    aNum,
				Timestamp: aTimestamp,
			},
			derived: types.BlockSeal{
				Hash:      common.BytesToHash(bHash),
				Number:    bNum,
				Timestamp: bTimestamp,
			},
		}
		entry := x.encode()
		require.Equal(t, SourceV0, entry.Type())
		var y LinkEntry
		err := y.decode(entry)
		require.NoError(t, err)
		require.Equal(t, x, y)
	})
}

func TestLinkEntry(t *testing.T) {
	t.Run("invalid type", func(t *testing.T) {
		var entry Entry
		entry[0] = 123
		var x LinkEntry
		require.ErrorContains(t, x.decode(entry), "unexpected")
	})
	t.Run("test type", func(t *testing.T) {
		link := LinkEntry{
			source: types.BlockSeal{
				Hash:      common.BytesToHash([]byte{1, 2, 3}),
				Number:    math.MaxUint64,
				Timestamp: math.MaxUint64,
			},
			derived: types.BlockSeal{
				Hash:      common.BytesToHash([]byte{4, 5, 6}),
				Number:    math.MaxUint64,
				Timestamp: math.MaxUint64,
			},
			entryType: InvalidatedFromV0,
		}
		require.False(t, link.Replacement())
		require.True(t, link.Invalidated())
		decoded := LinkEntry{}
		require.NoError(t, decoded.decode(link.encode()))
		require.False(t, decoded.Replacement())
		require.True(t, decoded.Invalidated())

		link.entryType = ReplacementV0
		require.True(t, link.Replacement())
		require.False(t, link.Invalidated())
		decoded = LinkEntry{}
		require.NoError(t, decoded.decode(link.encode()))
		require.True(t, decoded.Replacement())
		require.False(t, decoded.Invalidated())

		link.entryType = SourceV0
		require.False(t, link.Replacement())
		require.False(t, link.Invalidated())
		decoded = LinkEntry{}
		require.NoError(t, decoded.decode(link.encode()))
		require.False(t, decoded.Replacement())
		require.False(t, decoded.Invalidated())

		corrupt := link.encode()
		corrupt[0] = 17 // invalid type
		decoded = LinkEntry{}
		require.ErrorContains(t, decoded.decode(corrupt), "unexpected entry type")
	})
	t.Run("test length", func(t *testing.T) {
		link := LinkEntry{
			source: types.BlockSeal{
				Hash:      common.BytesToHash([]byte{1, 2, 3}),
				Number:    math.MaxUint64,
				Timestamp: math.MaxUint64,
			},
			derived: types.BlockSeal{
				Hash:      common.BytesToHash([]byte{4, 5, 6}),
				Number:    math.MaxUint64,
				Timestamp: math.MaxUint64,
			},
			entryType: InvalidatedFromV0,
		}
		require.Equal(t, EntrySize, len(link.encode()))
	})
}
