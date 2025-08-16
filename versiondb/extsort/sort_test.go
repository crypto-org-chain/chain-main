package extsort

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	ItemSize = 20
	ItemTpl  = "testkey-%d"
)

func doTestExtSorter(t *testing.T, chunkSize int64, inputCount int) {
	sorter := New("/tmp", Options{
		MaxChunkSize: chunkSize,
		LesserFunc: func(a, b []byte) bool {
			return bytes.Compare(a, b) == -1
		},
		DeltaEncoding:     true,
		SnappyCompression: true,
	})
	defer func() {
		require.NoError(t, sorter.Close())
	}()

	var expItems [][]byte
	for i := 0; i < inputCount; i++ {
		item := []byte(fmt.Sprintf(ItemTpl, i))
		expItems = append(expItems, item)
	}

	sort.Slice(expItems, func(i, j int) bool {
		return bytes.Compare(expItems[i], expItems[j]) == -1
	})

	randItems := make([][]byte, len(expItems))
	copy(randItems, expItems)
	g := rand.New(rand.NewSource(0))
	g.Shuffle(len(randItems), func(i, j int) { randItems[i], randItems[j] = randItems[j], randItems[i] })

	// feed in random order
	for _, item := range randItems {
		err := sorter.Feed(item)
		require.NoError(t, err)
	}
	reader, err := sorter.Finalize()
	require.NoError(t, err)

	var result [][]byte
	for {
		item, err := reader.Next()
		require.NoError(t, err)
		if item == nil {
			break
		}
		result = append(result, item)
	}

	require.Equal(t, expItems, result)
}

func TestExtSort(t *testing.T) {
	doTestExtSorter(t, ItemSize*100, 100)
	doTestExtSorter(t, ItemSize*100, 1550)
	doTestExtSorter(t, ItemSize*100, 155000)
}
