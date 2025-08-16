package rootmulti

import (
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store/types"
	"github.com/stretchr/testify/require"
)

func TestLastCommitID(t *testing.T) {
	store := NewStore(t.TempDir(), log.NewNopLogger(), false, false)
	require.Equal(t, types.CommitID{}, store.LastCommitID())
}
