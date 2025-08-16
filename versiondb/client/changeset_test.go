package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/cosmos/iavl"
	"github.com/stretchr/testify/require"
)

var ChangeSets []*iavl.ChangeSet

func init() {
	ChangeSets = append(ChangeSets,
		&iavl.ChangeSet{Pairs: []*iavl.KVPair{
			{Key: []byte("hello"), Value: []byte("world")},
		}},
		&iavl.ChangeSet{Pairs: []*iavl.KVPair{
			{Key: []byte("hello"), Value: []byte("world1")},
			{Key: []byte("hello1"), Value: []byte("world1")},
		}},
		&iavl.ChangeSet{Pairs: []*iavl.KVPair{
			{Key: []byte("hello2"), Value: []byte("world1")},
			{Key: []byte("hello3"), Value: []byte("world1")},
		}},
	)

	var changeSet iavl.ChangeSet
	for i := 0; i < 1; i++ {
		changeSet.Pairs = append(changeSet.Pairs, &iavl.KVPair{Key: []byte(fmt.Sprintf("hello%02d", i)), Value: []byte("world1")})
	}

	ChangeSets = append(ChangeSets, &changeSet)
	ChangeSets = append(ChangeSets, &iavl.ChangeSet{Pairs: []*iavl.KVPair{
		{Key: []byte("hello"), Delete: true},
		{Key: []byte("hello19"), Delete: true},
	}})

	changeSet = iavl.ChangeSet{}
	for i := 0; i < 21; i++ {
		changeSet.Pairs = append(changeSet.Pairs, &iavl.KVPair{Key: []byte(fmt.Sprintf("aello%02d", i)), Value: []byte("world1")})
	}
	ChangeSets = append(ChangeSets, &changeSet)

	changeSet = iavl.ChangeSet{}
	for i := 0; i < 21; i++ {
		changeSet.Pairs = append(changeSet.Pairs, &iavl.KVPair{Key: []byte(fmt.Sprintf("aello%02d", i)), Delete: true})
	}
	for i := 0; i < 19; i++ {
		changeSet.Pairs = append(changeSet.Pairs, &iavl.KVPair{Key: []byte(fmt.Sprintf("hello%02d", i)), Delete: true})
	}
	ChangeSets = append(ChangeSets, &changeSet)
}

func TestChangeSetEncoding(t *testing.T) {
	var buf bytes.Buffer
	for i, changeSet := range ChangeSets {
		version := int64(i + 1)
		require.NoError(t, WriteChangeSet(&buf, version, changeSet))
	}

	bufLen := buf.Len()
	offset, err := IterateChangeSets(&buf, func(version int64, changeSet *iavl.ChangeSet) (bool, error) {
		require.Equal(t, ChangeSets[version-1], changeSet)
		return true, nil
	})
	require.Equal(t, bufLen, int(offset))
	require.NoError(t, err)
}

func TestUvarintSize(t *testing.T) {
	var buf [binary.MaxVarintLen64]byte
	r := rand.New(rand.NewSource(0))
	for i := 0; i < 100; i++ {
		v := r.Uint64()
		n := binary.PutUvarint(buf[:], v)
		require.Equal(t, n, uvarintSize(v))
	}

	// test edge cases
	edgeCases := []uint64{0, math.MaxUint64}
	for _, v := range edgeCases {
		n := binary.PutUvarint(buf[:], v)
		require.Equal(t, n, uvarintSize(v))
	}
}

func TestKVPairRoundTrip(t *testing.T) {
	testCases := []struct {
		name string
		pair *iavl.KVPair
		pass bool
	}{
		{
			"update",
			&iavl.KVPair{Key: []byte("hello"), Value: []byte("world")},
			true,
		},
		{
			"delete",
			&iavl.KVPair{Key: []byte("hello"), Delete: true},
			true,
		},
		{
			"empty value",
			&iavl.KVPair{Key: []byte("hello"), Value: []byte{}},
			true,
		},
		{
			"uint16 key length don't overflow",
			&iavl.KVPair{Key: make([]byte, math.MaxUint16+1), Value: []byte{}},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := encodeKVPair(tc.pair)
			if tc.pass {
				require.NoError(t, err)
				require.Equal(t, encodedSizeOfKVPair(tc.pair), len(buf))

				pair, err := readKVPair(bytes.NewBuffer(buf))
				require.NoError(t, err)
				require.Equal(t, tc.pair, pair)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestChangeSetRoundTrip(t *testing.T) {
	testCases := []struct {
		name      string
		version   int64
		changeSet *iavl.ChangeSet
		pass      bool
	}{
		{
			"normal",
			1,
			&iavl.ChangeSet{
				Pairs: []*iavl.KVPair{
					{Key: []byte("hello"), Value: []byte("world")},
					{Key: []byte("hello"), Value: []byte{}},
					{Key: []byte("hello"), Value: []byte{}},
				},
			},
			true,
		},
		{
			"uint16 key length don't overflow",
			1,
			&iavl.ChangeSet{
				Pairs: []*iavl.KVPair{
					{Key: []byte("hello"), Value: []byte{}},
					{Key: []byte("hello"), Value: []byte{}},
					{Key: make([]byte, math.MaxUint16+1), Value: []byte{}},
				},
			},
			true,
		},
	}
	// TODO test key/value length overflow need to allocate
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.NewBuffer([]byte{})
			err := WriteChangeSet(buf, tc.version, tc.changeSet)
			encoded := buf.Bytes()
			if tc.pass {
				require.NoError(t, err)
				version, offset, changeSet, err := ReadChangeSet(bytes.NewReader(encoded), true)
				require.NoError(t, err)
				require.Equal(t, tc.version, version)
				require.Equal(t, tc.changeSet, changeSet)
				require.Equal(t, len(encoded), int(offset))
			} else {
				require.Error(t, err)
			}
		})
	}
}
