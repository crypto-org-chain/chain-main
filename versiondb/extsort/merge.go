package extsort

import (
	"container/heap"
)

// MultiWayMerge implements k-way merge using min-heap provided by golang builtin library,
// it implements `heap.Interface` interface.
type MultiWayMerge struct {
	entries    []*heapEntry
	lesserFunc LesserFunc
}

var _ heap.Interface = (*MultiWayMerge)(nil)

// NewMultiWayMerge initialize a new `MultiWayMerge` instance.
func NewMultiWayMerge(streams []NextFunc, lesserFunc LesserFunc) (*MultiWayMerge, error) {
	var entries []*heapEntry
	for _, stream := range streams {
		entry, err := newHeapEntry(stream)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
	}

	merge := &MultiWayMerge{
		entries:    entries,
		lesserFunc: lesserFunc,
	}

	heap.Init(merge)
	return merge, nil
}

// Len implements `heap.Interface`
func (merge *MultiWayMerge) Len() int {
	return len(merge.entries)
}

// Swap implements `heap.Interface`
func (merge *MultiWayMerge) Swap(i, j int) {
	merge.entries[i], merge.entries[j] = merge.entries[j], merge.entries[i]
}

// Less implements `heap.Interface`
func (merge *MultiWayMerge) Less(i, j int) bool {
	return merge.lesserFunc(merge.entries[i].value, merge.entries[j].value)
}

// Push implements `heap.Interface`
func (merge *MultiWayMerge) Push(x interface{}) {
	entry := x.(*heapEntry)
	merge.entries = append(merge.entries, entry)
}

// Pop implements `heap.Interface`
func (merge *MultiWayMerge) Pop() interface{} {
	l := merge.Len()
	item := merge.entries[l-1]
	merge.entries = merge.entries[:l-1]
	return item
}

// Next provides an iterator that yields sorted items
func (merge *MultiWayMerge) Next() ([]byte, error) {
	if merge.Len() == 0 {
		return nil, nil
	}
	minEntry := merge.entries[0]
	result := minEntry.value
	if err := minEntry.update(); err != nil {
		return nil, err
	}
	if minEntry.value == nil {
		heap.Remove(merge, 0)
	} else {
		heap.Fix(merge, 0)
	}
	return result, nil
}

// heapEntry is the min-heap entry represents a single sorted chunk.
type heapEntry struct {
	stream NextFunc
	value  []byte
}

// newHeapEntry initialize a `heapEntry`.
func newHeapEntry(stream NextFunc) (*heapEntry, error) {
	value, err := stream()
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	return &heapEntry{
		stream: stream,
		value:  value,
	}, nil
}

// update fetch the next item in the stream
func (entry *heapEntry) update() (err error) {
	entry.value, err = entry.stream()
	return
}
