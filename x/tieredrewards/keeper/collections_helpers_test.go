package keeper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections"
	"cosmossdk.io/collections/colltest"
)

func newTestKeySet(t *testing.T) (context.Context, collections.KeySet[collections.Pair[string, uint64]]) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	sb := collections.NewSchemaBuilder(storeService)
	ks := collections.NewKeySet(sb, collections.NewPrefix(0), "test",
		collections.PairKeyCodec(collections.StringKey, collections.Uint64Key))
	_, err := sb.Build()
	require.NoError(t, err)
	return ctx, ks
}

func TestCollectPairKeySetK2_Empty(t *testing.T) {
	ctx, ks := newTestKeySet(t)

	rng := collections.NewPrefixedPairRange[string, uint64]("prefix")
	vals, err := collectPairKeySetK2(ctx, ks, rng)
	require.NoError(t, err)
	require.Empty(t, vals)
}

func TestCollectPairKeySetK2_CollectsK2Values(t *testing.T) {
	ctx, ks := newTestKeySet(t)

	require.NoError(t, ks.Set(ctx, collections.Join("a", uint64(10))))
	require.NoError(t, ks.Set(ctx, collections.Join("a", uint64(20))))
	require.NoError(t, ks.Set(ctx, collections.Join("a", uint64(30))))
	require.NoError(t, ks.Set(ctx, collections.Join("b", uint64(40))))

	rng := collections.NewPrefixedPairRange[string, uint64]("a")
	vals, err := collectPairKeySetK2(ctx, ks, rng)
	require.NoError(t, err)
	require.Equal(t, []uint64{10, 20, 30}, vals)

	rng = collections.NewPrefixedPairRange[string, uint64]("b")
	vals, err = collectPairKeySetK2(ctx, ks, rng)
	require.NoError(t, err)
	require.Equal(t, []uint64{40}, vals)

	rng = collections.NewPrefixedPairRange[string, uint64]("z")
	vals, err = collectPairKeySetK2(ctx, ks, rng)
	require.NoError(t, err)
	require.Empty(t, vals)
}
