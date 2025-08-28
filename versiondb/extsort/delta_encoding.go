package extsort

import (
	"encoding/binary"
	"io"
)

// DeltaEncoder applies delta encoding to a sequence of keys
type DeltaEncoder struct {
	last []byte
}

func NewDeltaEncoder() *DeltaEncoder {
	return &DeltaEncoder{}
}

func (de *DeltaEncoder) Write(w io.Writer, key []byte) error {
	var sizeBuf [binary.MaxVarintLen64 + binary.MaxVarintLen64]byte

	shared := diffOffset(de.last, key)
	nonShared := len(key) - shared

	n1 := binary.PutUvarint(sizeBuf[:], uint64(shared))
	n2 := binary.PutUvarint(sizeBuf[n1:], uint64(nonShared))

	if _, err := w.Write(sizeBuf[:n1+n2]); err != nil {
		return err
	}

	if _, err := w.Write(key[shared:]); err != nil {
		return err
	}

	de.last = key
	return nil
}

// DeltaDecoder decodes delta-encoded keys
type DeltaDecoder struct {
	last []byte
}

func NewDeltaDecoder() *DeltaDecoder {
	return &DeltaDecoder{}
}

func (dd *DeltaDecoder) Read(
	reader reader,
) ([]byte, error) {
	shared, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}
	nonShared, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}

	item := make([]byte, shared+nonShared)
	copy(item[:shared], dd.last[:shared])
	if _, err := io.ReadFull(reader, item[shared:]); err != nil {
		return nil, err
	}

	dd.last = item
	return item, nil
}

type reader interface {
	io.Reader
	io.ByteReader
}

// diffOffset returns the index of first byte that's different in two bytes slice.
func diffOffset(a, b []byte) int {
	var off int
	var l int
	if len(a) < len(b) {
		l = len(a)
	} else {
		l = len(b)
	}
	for ; off < l; off++ {
		if a[off] != b[off] {
			break
		}
	}
	return off
}
