package client

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"os"
	"sort"
	"strings"

	"github.com/cosmos/iavl"
	"github.com/golang/snappy"
)

const (
	ZlibFileSuffix   = ".zz"
	SnappyFileSuffix = ".snappy"
)

// WriteChangeSet writes a version of change sets to writer.
//
// Change set file format:
// ```
// version: int64
// size: int64         // size of whole payload
// payload:
//
//	delete: int8
//	keyLen: varint-uint64
//	key
//	[ // if delete is false
//	  valueLen: varint-uint64
//	  value
//	]
//	repeat with next key-value pair
//
// repeat with next version
// ```
func WriteChangeSet(writer io.Writer, version int64, cs *iavl.ChangeSet) error {
	var size int
	items := make([][]byte, 0, len(cs.Pairs))
	for _, pair := range cs.Pairs {
		buf, err := encodeKVPair(pair)
		if err != nil {
			return err
		}
		size += len(buf)
		items = append(items, buf)
	}

	var versionHeader [16]byte
	binary.LittleEndian.PutUint64(versionHeader[:], uint64(version))
	binary.LittleEndian.PutUint64(versionHeader[8:], uint64(size))

	if _, err := writer.Write(versionHeader[:]); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := writer.Write(item); err != nil {
			return err
		}
	}
	return nil
}

// ReadChangeSet decode a version of change set from reader.
// if parseChangeset is false, it'll skip change set payload directly.
//
// returns (version, number of bytes read, changeSet, err)
func ReadChangeSet(reader Reader, parseChangeset bool) (int64, int64, *iavl.ChangeSet, error) {
	var versionHeader [16]byte
	if _, err := io.ReadFull(reader, versionHeader[:]); err != nil {
		return 0, 0, nil, err
	}
	version := binary.LittleEndian.Uint64(versionHeader[:8])
	size := int64(binary.LittleEndian.Uint64(versionHeader[8:16]))
	var changeSet iavl.ChangeSet

	if size == 0 {
		return int64(version), 16, &changeSet, nil
	}

	if parseChangeset {
		var offset int64
		for offset < size {
			pair, err := readKVPair(reader)
			if err != nil {
				return 0, 0, nil, err
			}
			offset += int64(encodedSizeOfKVPair(pair))
			changeSet.Pairs = append(changeSet.Pairs, pair)
		}
		if offset != size {
			return 0, 0, nil, fmt.Errorf("read beyond payload size limit, size: %d, offset: %d", size, offset)
		}
	} else {
		if _, err := io.CopyN(io.Discard, reader, size); err != nil {
			return 0, 0, nil, err
		}
	}
	return int64(version), size + 16, &changeSet, nil
}

// encodedSizeOfKVPair returns the encoded length of a key-value pair
//
// layout: deletion(1) + keyLen(varint) + key + [ valueLen(varint) + value ]
func encodedSizeOfKVPair(pair *iavl.KVPair) int {
	keyLen := len(pair.Key)
	size := 1 + uvarintSize(uint64(keyLen)) + keyLen
	if pair.Delete {
		return size
	}

	valueLen := len(pair.Value)
	return size + uvarintSize(uint64(valueLen)) + valueLen
}

// encodeKVPair encode a key-value pair in change set.
// see godoc of `encodedSizeOfKVPair` for layout description,
// returns error if key/value length overflows.
func encodeKVPair(pair *iavl.KVPair) ([]byte, error) {
	buf := make([]byte, encodedSizeOfKVPair(pair))

	offset := 1
	keyLen := len(pair.Key)
	offset += binary.PutUvarint(buf[offset:], uint64(keyLen))

	copy(buf[offset:], pair.Key)
	if pair.Delete {
		buf[0] = 1
		return buf, nil
	}

	offset += keyLen
	offset += binary.PutUvarint(buf[offset:], uint64(len(pair.Value)))
	copy(buf[offset:], pair.Value)
	return buf, nil
}

// Reader combines `io.Reader` and `io.ByteReader`.
type Reader interface {
	io.Reader
	io.ByteReader
}

// ReadCloser combines `Reader` and `io.Closer`.
type ReadCloser interface {
	Reader
	io.Closer
}

// readKVPair decode a key-value pair from reader
//
// see godoc of `encodedSizeOfKVPair` for layout description
func readKVPair(reader Reader) (*iavl.KVPair, error) {
	deletion, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	keyLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}

	pair := iavl.KVPair{
		Delete: deletion == 1,
		Key:    make([]byte, keyLen),
	}
	if _, err := io.ReadFull(reader, pair.Key); err != nil {
		return nil, err
	}

	if pair.Delete {
		return &pair, nil
	}

	valueLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}
	pair.Value = make([]byte, valueLen)
	if _, err := io.ReadFull(reader, pair.Value); err != nil {
		return nil, err
	}

	return &pair, nil
}

// openChangeSetFile opens change set file,
// it handles compressed files automatically,
// also supports special name "-" to specify stdin.
func openChangeSetFile(fileName string) (ReadCloser, error) {
	if fileName == "-" {
		return WrapReader(bufio.NewReader(os.Stdin), nil), nil
	}

	var reader Reader
	fp, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	switch {
	case strings.HasSuffix(fileName, ZlibFileSuffix):
		zreader, err := zlib.NewReader(fp)
		if err != nil {
			_ = fp.Close()
			return nil, err
		}
		reader = bufio.NewReader(zreader)
	case strings.HasSuffix(fileName, SnappyFileSuffix):
		reader = snappy.NewReader(fp)
	default:
		reader = bufio.NewReader(fp)
	}
	return WrapReader(reader, fp), nil
}

// withChangeSetFile opens change set file and pass the reader to callback,
// it closes the file immediately after callback returns.
func withChangeSetFile(fileName string, fn func(Reader) error) error {
	reader, err := openChangeSetFile(fileName)
	if err != nil {
		return err
	}
	defer reader.Close()
	return fn(reader)
}

// IterateChangeSets iterate the change set files,
func IterateChangeSets(
	reader Reader,
	fn func(version int64, changeSet *iavl.ChangeSet) (bool, error),
) (int64, error) {
	var (
		cont               = true
		lastCompleteOffset int64
		version, offset    int64
		changeSet          *iavl.ChangeSet
		err                error
	)

	for cont {
		version, offset, changeSet, err = ReadChangeSet(reader, true)
		if err != nil {
			break
		}
		cont, err = fn(version, changeSet)
		if err != nil {
			break
		}

		lastCompleteOffset += offset
	}

	if err == io.EOF {
		// it's not easy to distinguish normal EOF or unexpected EOF,
		// there could be potential corrupted end of file and the err is a normal io.EOF here,
		// user should verify the change set files in advance, using the verify command.
		err = nil
	}

	return lastCompleteOffset, err
}

// IterateVersions iterate the version numbers in change set files, skipping the change set payloads.
func IterateVersions(
	reader Reader,
	fn func(version int64) (bool, error),
) (int64, error) {
	var (
		cont               = true
		lastCompleteOffset int64
		version, offset    int64
		err                error
	)

	for cont {
		version, offset, _, err = ReadChangeSet(reader, false)
		if err != nil {
			break
		}
		cont, err = fn(version)
		if err != nil {
			break
		}

		lastCompleteOffset += offset
	}

	if err == io.EOF {
		// it's not easy to distinguish normal EOF or unexpected EOF,
		// there could be potential corrupted end of file and the err is a normal io.EOF here,
		// user should verify the change set files in advance, using the verify command.
		err = nil
	}

	return lastCompleteOffset, err
}

type FileWithVersion struct {
	FileName string
	Version  uint64
}

// SortFilesByFirstVerson parse the first version of the change set files and associate with the file name,
// then sort them by version number, also filter out empty files.
func SortFilesByFirstVerson(files []string) ([]FileWithVersion, error) {
	nonEmptyFiles := make([]FileWithVersion, 0, len(files))

	for _, fileName := range files {
		version, err := ReadFirstVersion(fileName)
		if err != nil {
			if err == io.EOF {
				// skipping empty files
				continue
			}
			return nil, err
		}

		nonEmptyFiles = append(nonEmptyFiles, FileWithVersion{
			FileName: fileName,
			Version:  version,
		})
	}

	sort.Slice(nonEmptyFiles, func(i, j int) bool {
		return nonEmptyFiles[i].Version < nonEmptyFiles[j].Version
	})
	return nonEmptyFiles, nil
}

// ReadFirstVersion parse the first version number in the change set file
func ReadFirstVersion(fileName string) (uint64, error) {
	fp, err := openChangeSetFile(fileName)
	if err != nil {
		return 0, err
	}
	defer fp.Close()

	// parse the first version number
	var versionBuf [8]byte
	_, err = io.ReadFull(fp, versionBuf[:])
	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint64(versionBuf[:]), nil
}

// uvarintSize returns the size (in bytes) of uint64 encoded with the `binary.PutUvarint`.
func uvarintSize(num uint64) int {
	bits := bits.Len64(num)
	q, r := bits/7, bits%7
	size := q
	if r > 0 || size == 0 {
		size++
	}
	return size
}
