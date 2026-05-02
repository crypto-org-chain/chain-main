package v7testnet

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PurgeOldTieredRewardsState wipes every key in the tieredrewards KV store
// whose prefix appears in `prefixes`. Callers typically pass StateToPurge()
// to preserve Params + Tiers, or AllPrefixes() for a complete wipe.
//
// The frozen pre-rewrite layout captured in this package (including
// ValidatorRewardRatioKeyPrefix at prefix 8, which no longer exists in
// types/keys.go) is what may still exist on-chain at upgrade time, so this
// function iterates those exact bytes rather than anything from
// x/tieredrewards/types.
//
// Pre-rewrite residue in staking (delegations held by the tier module account)
// and bank (balance at the module account) is NOT handled here — the caller
// must sweep those separately if required; on testnet, see
// sweepOldTierModuleResidualsTestnet in app/upgrades.go.
//
// Returns the deletion count per prefix (keyed by hex-formatted prefix bytes)
// for upgrade-time audit.
func PurgeOldTieredRewardsState(ctx sdk.Context, storeKey storetypes.StoreKey, prefixes [][]byte) (map[string]int, error) {
	store := ctx.KVStore(storeKey)
	counts := make(map[string]int, len(prefixes))
	logger := ctx.Logger().With("module", "x/tieredrewards", "upgrade", "v7.1.0-testnet")

	for _, prefix := range prefixes {
		// Collect keys first, delete after — iterator invalidation otherwise.
		iter := storetypes.KVStorePrefixIterator(store, prefix)
		var keys [][]byte
		for ; iter.Valid(); iter.Next() {
			k := make([]byte, len(iter.Key()))
			copy(k, iter.Key())
			keys = append(keys, k)
		}
		if err := iter.Close(); err != nil {
			return counts, fmt.Errorf("close iterator for prefix %x: %w", prefix, err)
		}

		for _, k := range keys {
			store.Delete(k)
		}

		label := fmt.Sprintf("%x", prefix)
		counts[label] = len(keys)
		logger.Info("purged tieredrewards prefix", "prefix", label, "count", len(keys))
	}

	return counts, nil
}
