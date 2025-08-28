package client

import (
	"errors"
	"io"
)

type wrapReader struct {
	reader Reader
	closer io.Closer
}

// WrapReader wraps reader and closer together to create a new io.ReadCloser.
//
// The Read function will simply call the wrapped reader's Read function,
// while the Close function will call the wrapped closer's Close function.
//
// If the wrapped reader is also an io.Closer,
// its Close function will be called in Close as well.
//
// closer can be `nil`, to support stdin.
func WrapReader(reader Reader, closer io.Closer) ReadCloser {
	return &wrapReader{
		reader: reader,
		closer: closer,
	}
}

func (r *wrapReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *wrapReader) ReadByte() (byte, error) {
	return r.reader.ReadByte()
}

func (r *wrapReader) Close() error {
	var errs []error
	if closer, ok := r.reader.(io.Closer); ok {
		errs = append(errs, closer.Close())
	}
	if r.closer != nil {
		errs = append(errs, r.closer.Close())
	}
	return errors.Join(errs...)
}
