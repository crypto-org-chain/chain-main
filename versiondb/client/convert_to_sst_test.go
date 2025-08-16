package client

import (
	"encoding/binary"
	"testing"

	"github.com/cosmos/iavl"
	"github.com/stretchr/testify/require"
)

func TestSorterItemEncoding(t *testing.T) {
	testCases := []struct {
		name    string
		version uint64
		pair    *iavl.KVPair
	}{
		{"default", 1, &iavl.KVPair{Delete: false, Key: []byte{1}, Value: []byte{1}}},
		{"delete", 1, &iavl.KVPair{Delete: true, Key: []byte{1}}},
		{"empty value", 1, &iavl.KVPair{Delete: false, Key: []byte{1}, Value: []byte{}}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := encodeSorterItem(tc.version, tc.pair)
			ts, p := decodeSorterItem(buf)
			v := binary.LittleEndian.Uint64(ts)
			require.Equal(t, tc.version, v)
			require.Equal(t, tc.pair, &p)
		})
	}
}

func TestSorterItemCompare(t *testing.T) {
	testCases := []struct {
		name     string
		version1 uint64
		pair1    *iavl.KVPair
		version2 uint64
		pair2    *iavl.KVPair
		result   bool
	}{
		{"lesser", 1, &iavl.KVPair{Key: []byte{1}}, 2, &iavl.KVPair{Key: []byte{2}}, true},
		{"equal", 1, &iavl.KVPair{Key: []byte{1}}, 1, &iavl.KVPair{Key: []byte{1}}, false},
		{"greater", 1, &iavl.KVPair{Key: []byte{2}}, 2, &iavl.KVPair{Key: []byte{1}}, false},
		{"smaller version", 1, &iavl.KVPair{Key: []byte{1}}, 2, &iavl.KVPair{Key: []byte{1}}, false},
		{"bigger version", 2, &iavl.KVPair{Key: []byte{1}}, 1, &iavl.KVPair{Key: []byte{1}}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bz1 := encodeSorterItem(tc.version1, tc.pair1)
			bz2 := encodeSorterItem(tc.version2, tc.pair2)
			require.Equal(t, tc.result, compareSorterItem(bz1, bz2))
		})
	}
}
