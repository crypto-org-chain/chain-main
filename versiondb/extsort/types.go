package extsort

type (
	// LesserFunc compares two items in external sorter
	LesserFunc func(a, b []byte) bool
	// NextFunc is a stream that yields bytes, it should return `nil` when stream is exhausted
	NextFunc func() ([]byte, error)
)
