package extsort

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeltaEncoding(t *testing.T) {
	testCases := []struct {
		name    string
		keys    []string
		expSize int
	}{
		{
			"no share",
			[]string{"hello", "world"},
			14,
		},
		{
			"share",
			[]string{"hello", "hello2"},
			10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoder := NewDeltaEncoder()
			var buf bytes.Buffer
			for _, key := range tc.keys {
				require.NoError(t, encoder.Write(&buf, []byte(key)))
			}
			require.Equal(t, tc.expSize, buf.Len())

			decoder := NewDeltaDecoder()
			var keys []string
			for {
				key, err := decoder.Read(&buf)
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				keys = append(keys, string(key))
			}
			require.Equal(t, tc.keys, keys)
		})
	}
}
