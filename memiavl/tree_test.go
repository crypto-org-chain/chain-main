package memiavl

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store/wrapper"
	db "github.com/cosmos/cosmos-db"
	"github.com/cosmos/iavl"
	"github.com/stretchr/testify/require"
)

var (
	ChangeSets  []ChangeSet
	RefHashes   [][]byte
	ExpectItems [][]pair

	IAVLInitialVersion      = 100
	RefHashesInitialVersion [][]byte
)

func mockKVPairs(kvPairs ...string) []*KVPair {
	result := make([]*KVPair, len(kvPairs)/2)
	for i := 0; i < len(kvPairs); i += 2 {
		result[i/2] = &KVPair{
			Key:   []byte(kvPairs[i]),
			Value: []byte(kvPairs[i+1]),
		}
	}
	return result
}

func init() {
	ChangeSets = []ChangeSet{
		{Pairs: mockKVPairs("hello", "world")},
		{Pairs: mockKVPairs("hello", "world1", "hello1", "world1")},
		{Pairs: mockKVPairs("hello2", "world1", "hello3", "world1")},
	}

	changes := ChangeSet{}
	for i := 0; i < 1; i++ {
		changes.Pairs = append(changes.Pairs, &KVPair{Key: []byte(fmt.Sprintf("hello%02d", i)), Value: []byte("world1")})
	}

	ChangeSets = append(ChangeSets, changes)
	ChangeSets = append(ChangeSets, ChangeSet{Pairs: []*KVPair{{Key: []byte("hello"), Delete: true}, {Key: []byte("hello19"), Delete: true}}})

	changes = ChangeSet{}
	for i := 0; i < 21; i++ {
		changes.Pairs = append(changes.Pairs, &KVPair{Key: []byte(fmt.Sprintf("aello%02d", i)), Value: []byte("world1")})
	}
	ChangeSets = append(ChangeSets, changes)

	changes = ChangeSet{}
	for i := 0; i < 21; i++ {
		changes.Pairs = append(changes.Pairs, &KVPair{Key: []byte(fmt.Sprintf("aello%02d", i)), Delete: true})
	}
	for i := 0; i < 19; i++ {
		changes.Pairs = append(changes.Pairs, &KVPair{Key: []byte(fmt.Sprintf("hello%02d", i)), Delete: true})
	}
	ChangeSets = append(ChangeSets, changes)

	// generate ref hashes with ref impl
	d := wrapper.NewDBWrapper(db.NewMemDB())
	refTree := iavl.NewMutableTree(d, 0, true, log.NewNopLogger())
	refTreeInitialVersion := iavl.NewMutableTree(d, 0, true, log.NewNopLogger(), iavl.InitialVersionOption(uint64(IAVLInitialVersion)))
	for _, changes := range ChangeSets {
		{
			if err := applyChangeSetRef(refTreeInitialVersion, changes); err != nil {
				panic(err)
			}
			workingHash := refTreeInitialVersion.WorkingHash()
			refHash, _, err := refTreeInitialVersion.SaveVersion()
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(workingHash, refHash) {
				panic(fmt.Sprintf("working hash %X != ref hash %X", workingHash, refHash))
			}
			RefHashesInitialVersion = append(RefHashesInitialVersion, refHash)
		}

		if err := applyChangeSetRef(refTree, changes); err != nil {
			panic(err)
		}
		workingHash := refTree.WorkingHash()
		refHash, _, err := refTree.SaveVersion()
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(workingHash, refHash) {
			panic(fmt.Sprintf("working hash %X != ref hash %X", workingHash, refHash))
		}
		RefHashes = append(RefHashes, refHash)
	}

	ExpectItems = [][]pair{
		{},
		{{[]byte("hello"), []byte("world")}},
		{
			{[]byte("hello"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
		},
		{
			{[]byte("hello"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
		{
			{[]byte("hello"), []byte("world1")},
			{[]byte("hello00"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
		{
			{[]byte("hello00"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
		{
			{[]byte("aello00"), []byte("world1")},
			{[]byte("aello01"), []byte("world1")},
			{[]byte("aello02"), []byte("world1")},
			{[]byte("aello03"), []byte("world1")},
			{[]byte("aello04"), []byte("world1")},
			{[]byte("aello05"), []byte("world1")},
			{[]byte("aello06"), []byte("world1")},
			{[]byte("aello07"), []byte("world1")},
			{[]byte("aello08"), []byte("world1")},
			{[]byte("aello09"), []byte("world1")},
			{[]byte("aello10"), []byte("world1")},
			{[]byte("aello11"), []byte("world1")},
			{[]byte("aello12"), []byte("world1")},
			{[]byte("aello13"), []byte("world1")},
			{[]byte("aello14"), []byte("world1")},
			{[]byte("aello15"), []byte("world1")},
			{[]byte("aello16"), []byte("world1")},
			{[]byte("aello17"), []byte("world1")},
			{[]byte("aello18"), []byte("world1")},
			{[]byte("aello19"), []byte("world1")},
			{[]byte("aello20"), []byte("world1")},
			{[]byte("hello00"), []byte("world1")},
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
		{
			{[]byte("hello1"), []byte("world1")},
			{[]byte("hello2"), []byte("world1")},
			{[]byte("hello3"), []byte("world1")},
		},
	}
}

func applyChangeSetRef(t *iavl.MutableTree, changes ChangeSet) error {
	for _, change := range changes.Pairs {
		if change.Delete {
			if _, _, err := t.Remove(change.Key); err != nil {
				return err
			}
		} else {
			if _, err := t.Set(change.Key, change.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestRootHashes(t *testing.T) {
	tree := New(0)

	for i, changes := range ChangeSets {
		tree.ApplyChangeSet(changes)
		workingHash := tree.RootHash()
		hash, v, err := tree.SaveVersion(true)
		require.NoError(t, err)
		require.Equal(t, i+1, int(v))
		require.Equal(t, RefHashes[i], hash)
		require.Equal(t, hash, workingHash)
	}
}

func TestRootHashesInitialVersion(t *testing.T) {
	tree := NewWithInitialVersion(uint32(IAVLInitialVersion), 0)

	for i, changes := range ChangeSets {
		tree.ApplyChangeSet(changes)
		workingHash := tree.RootHash()
		hash, v, err := tree.SaveVersion(true)
		require.NoError(t, err)
		require.Equal(t, IAVLInitialVersion+i, int(v))
		require.Equal(t, RefHashesInitialVersion[i], hash)
		require.Equal(t, hash, workingHash)
	}
}

func TestNewKey(t *testing.T) {
	tree := New(0)

	for i := 0; i < 4; i++ {
		tree.set([]byte(fmt.Sprintf("key-%d", i)), []byte{1})
	}
	_, _, err := tree.SaveVersion(true)
	require.NoError(t, err)

	// the smallest key in the right half of the tree
	require.Equal(t, tree.root.Key(), []byte("key-2"))

	// remove this key
	tree.remove([]byte("key-2"))

	// check root node's key is changed
	require.Equal(t, []byte("key-3"), tree.root.Key())
}

func TestEmptyTree(t *testing.T) {
	tree := New(0)
	require.Equal(t, emptyHash, tree.RootHash())
}

func TestTreeCopy(t *testing.T) {
	tree := New(0)

	tree.ApplyChangeSet(ChangeSet{Pairs: []*KVPair{
		{Key: []byte("hello"), Value: []byte("world")},
	}})
	_, _, err := tree.SaveVersion(true)
	require.NoError(t, err)

	snapshot := tree.Copy(0)

	tree.ApplyChangeSet(ChangeSet{Pairs: []*KVPair{
		{Key: []byte("hello"), Value: []byte("world1")},
	}})
	_, _, err = tree.SaveVersion(true)
	require.NoError(t, err)

	require.Equal(t, []byte("world1"), tree.Get([]byte("hello")))
	require.Equal(t, []byte("world"), snapshot.Get([]byte("hello")))

	// check that normal copy don't work
	fakeSnapshot := *tree

	tree.ApplyChangeSet(ChangeSet{Pairs: []*KVPair{
		{Key: []byte("hello"), Value: []byte("world2")},
	}})
	_, _, err = tree.SaveVersion(true)
	require.NoError(t, err)

	// get modified in-place
	require.Equal(t, []byte("world2"), tree.Get([]byte("hello")))
	require.Equal(t, []byte("world2"), fakeSnapshot.Get([]byte("hello")))
}

func TestChangeSetMarshal(t *testing.T) {
	for _, changes := range ChangeSets {
		bz, err := changes.Marshal()
		require.NoError(t, err)

		var cs ChangeSet
		require.NoError(t, cs.Unmarshal(bz))
		require.Equal(t, changes, cs)
	}
}

func TestGetByIndex(t *testing.T) {
	changes := ChangeSet{}
	for i := 0; i < 20; i++ {
		changes.Pairs = append(changes.Pairs, &KVPair{Key: []byte(fmt.Sprintf("hello%02d", i)), Value: []byte(strconv.Itoa(i))})
	}

	tree := New(0)
	tree.ApplyChangeSet(changes)
	_, _, err := tree.SaveVersion(true)
	require.NoError(t, err)

	for i, pair := range changes.Pairs {
		idx, v := tree.GetWithIndex(pair.Key)
		require.Equal(t, pair.Value, v)
		require.Equal(t, int64(i), idx)

		k, v := tree.GetByIndex(idx)
		require.Equal(t, pair.Key, k)
		require.Equal(t, pair.Value, v)
	}

	// test persisted tree
	dir := t.TempDir()
	require.NoError(t, tree.WriteSnapshot(dir))
	snapshot, err := OpenSnapshot(dir)
	require.NoError(t, err)
	ptree := NewFromSnapshot(snapshot, true, 0)
	defer ptree.Close()

	for i, pair := range changes.Pairs {
		idx, v := ptree.GetWithIndex(pair.Key)
		require.Equal(t, pair.Value, v)
		require.Equal(t, int64(i), idx)

		k, v := ptree.GetByIndex(idx)
		require.Equal(t, pair.Key, k)
		require.Equal(t, pair.Value, v)
	}
}
