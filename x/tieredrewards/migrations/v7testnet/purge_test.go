package v7testnet_test

import (
	"bytes"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	v7testnet "github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/migrations/v7testnet"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// newTestStore spins up a minimal in-memory multi-store with a single KV
// store mounted under "tieredrewards", returning the store key and a fresh
// sdk.Context. Avoids importing the whole app wiring.
func newTestStore(t *testing.T) (sdk.Context, *storetypes.KVStoreKey) {
	t.Helper()
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	key := storetypes.NewKVStoreKey("tieredrewards")
	ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, cmtproto.Header{}, false, log.NewNopLogger())
	return ctx, key
}

// seed writes value v at key = prefix || suffix.
func seed(store storetypes.KVStore, prefix, suffix, v []byte) {
	k := make([]byte, 0, len(prefix)+len(suffix))
	k = append(k, prefix...)
	k = append(k, suffix...)
	store.Set(k, v)
}

func TestPurgeOldTieredRewardsState_AllPrefixes(t *testing.T) {
	ctx, key := newTestStore(t)
	store := ctx.KVStore(key)

	// Seed two entries under every frozen prefix.
	prefixes := v7testnet.AllPrefixes()
	for _, p := range prefixes {
		seed(store, p, []byte{0x01}, []byte("a"))
		seed(store, p, []byte{0x02}, []byte("b"))
	}
	// Also seed an out-of-band key that is NOT under any frozen prefix. Pick a
	// prefix byte guaranteed to lie past the highest frozen prefix so it won't
	// collide with the purge.
	highest := byte(0)
	for _, p := range prefixes {
		if p[0] > highest {
			highest = p[0]
		}
	}
	foreign := []byte{highest + 1, 0xFF}
	store.Set(foreign, []byte("keep-me"))

	counts, err := v7testnet.PurgeOldTieredRewardsState(ctx, key, prefixes)
	require.NoError(t, err)

	// Every prefix should report two deletions.
	require.Len(t, counts, len(prefixes))
	for _, p := range prefixes {
		require.Equal(t, 2, counts[string(formatPrefix(p))],
			"prefix %x should have 2 deletions", p)
	}

	// Every purged key should be gone.
	for _, p := range prefixes {
		iter := storetypes.KVStorePrefixIterator(store, p)
		hasKey := iter.Valid()
		require.NoError(t, iter.Close())
		require.False(t, hasKey, "prefix %x should be empty after purge", p)
	}

	// Foreign key must survive.
	require.True(t, bytes.Equal(store.Get(foreign), []byte("keep-me")),
		"out-of-band key must survive purge")
}

// TestPurgeOldTieredRewardsState_StateToPurge_PreservesParamsAndTiers seeds
// entries under every prefix including Params and Tiers, calls the purge with
// StateToPurge(), and verifies that Params + Tiers bytes survive while every
// other prefix is wiped.
func TestPurgeOldTieredRewardsState_StateToPurge_PreservesParamsAndTiers(t *testing.T) {
	ctx, key := newTestStore(t)
	store := ctx.KVStore(key)

	for _, p := range v7testnet.AllPrefixes() {
		seed(store, p, []byte{0x01}, []byte("a"))
	}

	counts, err := v7testnet.PurgeOldTieredRewardsState(ctx, key, v7testnet.StateToPurge())
	require.NoError(t, err)

	// StateToPurge must NOT include Params or Tiers.
	require.NotContains(t, counts, string(formatPrefix(v7testnet.ParamsKeyPrefix)),
		"StateToPurge should not touch Params")
	require.NotContains(t, counts, string(formatPrefix(v7testnet.TiersKeyPrefix)),
		"StateToPurge should not touch Tiers")

	// Params + Tiers bytes must survive.
	for _, p := range [][]byte{v7testnet.ParamsKeyPrefix, v7testnet.TiersKeyPrefix} {
		iter := storetypes.KVStorePrefixIterator(store, p)
		hasKey := iter.Valid()
		require.NoError(t, iter.Close())
		require.True(t, hasKey, "prefix %x (Params/Tiers) must survive StateToPurge", p)
	}

	// Everything in StateToPurge must be empty.
	for _, p := range v7testnet.StateToPurge() {
		iter := storetypes.KVStorePrefixIterator(store, p)
		hasKey := iter.Valid()
		require.NoError(t, iter.Close())
		require.False(t, hasKey, "prefix %x should be empty after StateToPurge", p)
	}
}

func TestPurgeOldTieredRewardsState_EmptyStore(t *testing.T) {
	ctx, key := newTestStore(t)

	counts, err := v7testnet.PurgeOldTieredRewardsState(ctx, key, v7testnet.AllPrefixes())
	require.NoError(t, err)

	for _, p := range v7testnet.AllPrefixes() {
		require.Equal(t, 0, counts[string(formatPrefix(p))],
			"prefix %x should report 0 deletions on empty store", p)
	}
}

// formatPrefix mirrors the label-formatting inside PurgeOldTieredRewardsState
// (fmt.Sprintf("%x", prefix)) so test assertions can look up counts by the
// same key the function emits.
func formatPrefix(p []byte) []byte {
	const hex = "0123456789abcdef"
	out := make([]byte, 0, len(p)*2)
	for _, b := range p {
		out = append(out, hex[b>>4], hex[b&0x0F])
	}
	return out
}
