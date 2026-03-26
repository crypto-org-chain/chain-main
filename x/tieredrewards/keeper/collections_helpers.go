package keeper

import (
	"context"

	"cosmossdk.io/collections"
)

// collectPairKeySetK2 extracts K2 values from a KeySet[Pair[K1, K2]] iterator.
func collectPairKeySetK2[K1, K2 any](ctx context.Context, keySet collections.KeySet[collections.Pair[K1, K2]], rng *collections.PairRange[K1, K2]) ([]K2, error) {
	iter, err := keySet.Iterate(ctx, rng)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var values []K2
	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return nil, err
		}
		values = append(values, key.K2())
	}
	return values, nil
}
