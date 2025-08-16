// Package extsort implements external sorting algorithm, it has several differnet design choices compared with alternatives like https://github.com/lanrat/extsort:
//   - apply efficient compressions(delta encoding + snappy) to the chunk files to reduce IO cost,
//     since the items are sorted, delta encoding should be effective to it, and snappy is pretty efficient.
//   - chunks are stored in separate temporary files, so the chunk sorting and saving can run in parallel (eats more ram though).
//   - clean interface, user just feed `[]byte` directly, and provides a compare function based on `[]byte`.
package extsort

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/golang/snappy"
)

// Options defines configurable options for `ExtSorter`
type Options struct {
	// if apply delta encoding to the items in sorted chunk
	DeltaEncoding bool
	// if apply snappy compression to the items in sorted chunk
	SnappyCompression bool
	// Maxiumum uncompressed size of the sorted chunk
	MaxChunkSize int64
	// function that compares two items
	LesserFunc LesserFunc
}

// ExtSorter implements external sorting.
// It split the inputs into chunks, sort each chunk separately and save to disk file,
// then provide an iterator of sorted items by doing a k-way merge with all the sorted chunk files.
type ExtSorter struct {
	opts Options

	// directory to store temporary chunk files
	tmpDir string

	// current chunk
	currentChunk     [][]byte
	currentChunkSize int64

	// manage the chunk goroutines
	chunkWG sync.WaitGroup
	lock    sync.Mutex
	// finished chunk files
	chunkFiles []*os.File
	// chunking goroutine failure messages
	failures []string
}

// New creates a new `ExtSorter`.
func New(tmpDir string, opts Options) *ExtSorter {
	return &ExtSorter{
		tmpDir: tmpDir,
		opts:   opts,
	}
}

// Spawn spawns a new goroutine to do the external sorting,
// returns two channels for user to interact.
func Spawn(tmpDir string, opts Options, bufferSize int) (chan []byte, chan []byte) {
	inputChan := make(chan []byte, bufferSize)
	outputChan := make(chan []byte, bufferSize)

	go func() {
		defer close(outputChan)

		sorter := New(tmpDir, opts)
		defer sorter.Close()

		for bz := range inputChan {
			if err := sorter.Feed(bz); err != nil {
				panic(err)
			}
		}

		reader, err := sorter.Finalize()
		if err != nil {
			panic(err)
		}

		for {
			item, err := reader.Next()
			if err != nil {
				panic(err)
			}
			if item == nil {
				break
			}

			outputChan <- item
		}
	}()

	return inputChan, outputChan
}

// Feed add un-ordered items to the sorter.
func (s *ExtSorter) Feed(item []byte) error {
	if len(item) > math.MaxUint32 {
		return errors.New("item length overflows uint32")
	}

	s.currentChunkSize += int64(len(item)) + 4
	s.currentChunk = append(s.currentChunk, item)

	if s.currentChunkSize >= s.opts.MaxChunkSize {
		return s.sortChunkAndRotate()
	}
	return nil
}

// sortChunkAndRotate sort the current chunk and save to disk.
func (s *ExtSorter) sortChunkAndRotate() error {
	chunkFile, err := os.CreateTemp(s.tmpDir, "sort-chunk-*")
	if err != nil {
		return err
	}

	// rotate chunk
	chunk := s.currentChunk
	s.currentChunk = nil
	s.currentChunkSize = 0

	s.chunkWG.Add(1)
	go func() {
		defer s.chunkWG.Done()
		if err := s.sortAndSaveChunk(chunk, chunkFile); err != nil {
			chunkFile.Close()
			s.lock.Lock()
			defer s.lock.Unlock()
			s.failures = append(s.failures, err.Error())
			return
		}
		s.lock.Lock()
		defer s.lock.Unlock()
		s.chunkFiles = append(s.chunkFiles, chunkFile)
	}()
	return nil
}

// Finalize wait for all chunking goroutines to finish, and return the merged sorted stream.
func (s *ExtSorter) Finalize() (*MultiWayMerge, error) {
	// handle the pending chunk
	if s.currentChunkSize > 0 {
		if err := s.sortChunkAndRotate(); err != nil {
			return nil, err
		}
	}

	s.chunkWG.Wait()
	if len(s.failures) > 0 {
		return nil, errors.New(strings.Join(s.failures, "\n"))
	}

	streams := make([]NextFunc, len(s.chunkFiles))
	for i, chunkFile := range s.chunkFiles {
		if _, err := chunkFile.Seek(0, 0); err != nil {
			return nil, err
		}

		var reader reader
		if s.opts.SnappyCompression {
			reader = snappy.NewReader(chunkFile)
		} else {
			reader = bufio.NewReader(chunkFile)
		}

		if s.opts.DeltaEncoding {
			decoder := NewDeltaDecoder()
			streams[i] = func() ([]byte, error) {
				item, err := decoder.Read(reader)
				if err == io.EOF {
					return nil, nil
				}
				return item, err
			}
		} else {
			streams[i] = func() ([]byte, error) {
				size, err := binary.ReadUvarint(reader)
				if err != nil {
					if err == io.EOF && size == 0 {
						return nil, nil
					}
					return nil, err
				}
				item := make([]byte, size)
				if _, err := io.ReadFull(reader, item); err != nil {
					return nil, err
				}
				return item, nil
			}
		}
	}

	return NewMultiWayMerge(streams, s.opts.LesserFunc)
}

// Close closes and remove all the temporary chunk files
func (s *ExtSorter) Close() error {
	var errs []error
	for _, chunkFile := range s.chunkFiles {
		errs = append(errs, chunkFile.Close(), os.Remove(chunkFile.Name()))
	}
	return errors.Join(errs...)
}

type bufWriter interface {
	io.Writer
	Flush() error
}

// sortAndSaveChunk sort the chunk in memory and save to disk in order,
// it applies delta encoding and snappy compression to the items.
func (s *ExtSorter) sortAndSaveChunk(chunk [][]byte, output *os.File) error {
	// sort the chunk and write to file
	sort.Slice(chunk, func(i, j int) bool {
		return s.opts.LesserFunc(chunk[i], chunk[j])
	})

	var writer bufWriter
	if s.opts.SnappyCompression {
		writer = snappy.NewBufferedWriter(output)
	} else {
		writer = bufio.NewWriter(output)
	}

	if s.opts.DeltaEncoding {
		encoder := NewDeltaEncoder()
		for _, item := range chunk {
			if err := encoder.Write(writer, item); err != nil {
				return err
			}
		}
	} else {
		for _, item := range chunk {
			var buf [binary.MaxVarintLen64]byte
			n := binary.PutUvarint(buf[:], uint64(len(item)))
			if _, err := writer.Write(buf[:n]); err != nil {
				return err
			}
			if _, err := writer.Write(item); err != nil {
				return err
			}
		}
	}
	return writer.Flush()
}
