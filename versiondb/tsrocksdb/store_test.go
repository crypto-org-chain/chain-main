package tsrocksdb

import (
	"encoding/binary"
	"testing"

	"github.com/crypto-org-chain/cronos/versiondb"
	"github.com/linxGnu/grocksdb"
	"github.com/stretchr/testify/require"
)

func TestTSVersionDB(t *testing.T) {
	versiondb.Run(t, func() versiondb.VersionStore {
		store, err := NewStore(t.TempDir())
		require.NoError(t, err)
		return store
	})
}

// TestUserTimestamp tests the behaviors of user-defined timestamp feature of rocksdb
func TestUserTimestampBasic(t *testing.T) {
	key := []byte("hello")
	writeOpts := grocksdb.NewDefaultWriteOptions()

	db, cfHandle, err := OpenVersionDB(t.TempDir())
	require.NoError(t, err)

	var ts [8]byte
	binary.LittleEndian.PutUint64(ts[:], 1000)

	err = db.PutCFWithTS(writeOpts, cfHandle, key, ts[:], []byte{1})
	require.NoError(t, err)
	err = db.PutCFWithTS(writeOpts, cfHandle, []byte("zempty"), ts[:], []byte{})
	require.NoError(t, err)

	// key don't exists in older version
	v := int64(999)
	bz, err := db.GetCF(newTSReadOptions(&v), cfHandle, key)
	require.NoError(t, err)
	require.False(t, bz.Exists())
	bz.Free()

	// key exists in latest version
	bz, err = db.GetCF(newTSReadOptions(nil), cfHandle, key)
	require.NoError(t, err)
	require.Equal(t, []byte{1}, bz.Data())
	bz.Free()

	// iterator can find the key in right version
	v = int64(1000)
	it := db.NewIteratorCF(newTSReadOptions(&v), cfHandle)
	it.SeekToFirst()
	require.True(t, it.Valid())
	bz = it.Key()
	require.Equal(t, key, bz.Data())
	bz.Free()

	// key exists in right version, and empty value is supported
	bz, err = db.GetCF(newTSReadOptions(&v), cfHandle, []byte("zempty"))
	require.NoError(t, err)
	require.Equal(t, []byte{}, bz.Data())
	bz.Free()

	binary.LittleEndian.PutUint64(ts[:], 1002)
	err = db.PutCFWithTS(writeOpts, cfHandle, []byte("hella"), ts[:], []byte{2})
	require.NoError(t, err)

	// iterator can find keys from both versions
	v = int64(1002)
	it = db.NewIteratorCF(newTSReadOptions(&v), cfHandle)
	it.SeekToFirst()
	require.True(t, it.Valid())
	bz = it.Key()
	require.Equal(t, []byte("hella"), bz.Data())
	bz.Free()

	it.Next()
	require.True(t, it.Valid())
	bz = it.Key()
	require.Equal(t, key, bz.Data())
	bz.Free()

	for i := 1; i < 100; i++ {
		binary.LittleEndian.PutUint64(ts[:], uint64(i))
		err := db.PutCFWithTS(writeOpts, cfHandle, key, ts[:], []byte{byte(i)})
		require.NoError(t, err)
	}

	for i := int64(1); i < 100; i++ {
		binary.LittleEndian.PutUint64(ts[:], uint64(i))
		bz, err := db.GetCF(newTSReadOptions(&i), cfHandle, key)
		require.NoError(t, err)
		require.Equal(t, []byte{byte(i)}, bz.Data())
		bz.Free()
	}
}

func TestUserTimestampPruning(t *testing.T) {
	key := []byte("hello")
	writeOpts := grocksdb.NewDefaultWriteOptions()

	dir := t.TempDir()
	db, cfHandle, err := OpenVersionDB(dir)
	require.NoError(t, err)

	var ts [TimestampSize]byte
	for _, i := range []uint64{1, 100, 200} {
		binary.LittleEndian.PutUint64(ts[:], i)
		err := db.PutCFWithTS(writeOpts, cfHandle, key, ts[:], []byte{byte(i)})
		require.NoError(t, err)
	}

	i := int64(49)

	bz, err := db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.NoError(t, err)
	require.True(t, bz.Exists())
	bz.Free()

	// prune old versions
	binary.LittleEndian.PutUint64(ts[:], 50)
	compactOpts := grocksdb.NewCompactRangeOptions()
	compactOpts.SetFullHistoryTsLow(ts[:])
	db.CompactRangeCFOpt(cfHandle, grocksdb.Range{}, compactOpts)

	// queries for versions older than 50 are not allowed
	_, err = db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.Error(t, err)

	// the value previously at version 1 is still there
	i = 50
	bz, err = db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.NoError(t, err)
	require.True(t, bz.Exists())
	require.Equal(t, []byte{1}, bz.Data())
	bz.Free()

	i = 200
	bz, err = db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.NoError(t, err)
	require.Equal(t, []byte{200}, bz.Data())
	bz.Free()

	// reopen db and trim version 200
	cfHandle.Destroy()
	db.Close()
	db, cfHandle, err = OpenVersionDBAndTrimHistory(dir, 199)
	require.NoError(t, err)

	// the version 200 is gone, 100 is the latest value
	bz, err = db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.NoError(t, err)
	require.Equal(t, []byte{100}, bz.Data())
	bz.Free()
}
