package keeper_test

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/crypto-org-chain/chain-main/v8/testutil"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/keeper"
	"github.com/crypto-org-chain/chain-main/v8/x/tieredrewards/types"

	sdkmath "cosmossdk.io/math"

	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
)

// blockMaxGas mirrors the chain's production block gas limit.
const blockMaxGas = uint64(81_500_000)

// TestMsgClaimTierRewards_GasBenchmark measures gas consumption of a
// MsgClaimTierRewards transaction end-to-end — from the AnteHandler
// (tx size, sig verification, fee deduction) through the msg handler —
// for several batch sizes. It exists to justify the MaxClaimPositionIds
// value against the chain's block gas limit.
//
// Baseline: no prior validator slashes, fresh validator state — the cheapest
// path through ClaimTierRewards.
//
// Run with:
//
//	go test -v ./x/tieredrewards/keeper/... -run TestKeeperSuite/TestMsgClaimTierRewards_GasBenchmark$
func (s *KeeperSuite) TestMsgClaimTierRewards_GasBenchmark() {
	s.runGasBenchmarkTable("baseline (0 slashes)", 0)
}

// TestMsgClaimTierRewards_GasBenchmark_WorstCase measures gas under a
// pessimistic but plausible scenario: the validator has been slashed
// several times since the positions were created. Each slash:
//
//  1. Appends a ValidatorEvent that every position must walk in
//     processEventsAndClaimBonus and decrement the ref count on.
//  2. Creates a distribution ValidatorSlashEvent that
//     WithdrawDelegationRewards iterates for every delegation.
//
// Both scale per-position gas linearly in the number of slashes.
//
// Run with:
//
//	go test -v ./x/tieredrewards/keeper/... -run TestKeeperSuite/TestMsgClaimTierRewards_GasBenchmark_WorstCase
func (s *KeeperSuite) TestMsgClaimTierRewards_GasBenchmark_WorstCase() {
	// 10 slashes is an aggressive upper bound — real validators typically
	// see 0–2 downtime slashes per year.
	s.runGasBenchmarkTable("worst case (10 slashes)", 10)
}

func (s *KeeperSuite) runGasBenchmarkTable(label string, numSlashes int) {
	batchSizes := []int{1, 10, 50, 100, 200, 300}

	s.T().Logf("scenario: %s", label)
	s.T().Logf("block max gas: %d", blockMaxGas)
	s.T().Logf("%-6s %-14s %-14s %-12s %s", "N", "gasUsed", "perPosition", "%ofBlock", "fitsInBlock")

	for _, n := range batchSizes {
		s.Run(fmt.Sprintf("n=%d", n), func() {
			s.SetupTest()
			gasUsed := s.runClaimRewardsGasBenchmark(n, numSlashes)
			perPos := gasUsed / uint64(n)
			pctBlock := 100.0 * float64(gasUsed) / float64(blockMaxGas)
			fits := "YES"
			if gasUsed > blockMaxGas {
				fits = fmt.Sprintf("NO (%.1fx over)", float64(gasUsed)/float64(blockMaxGas))
			}
			s.T().Logf("%-6d %-14d %-14d %-12.2f %s", n, gasUsed, perPos, pctBlock, fits)
		})
	}
}

// runClaimRewardsGasBenchmark creates n delegated positions owned by a single
// signing account, optionally slashes the validator numSlashes times to
// accumulate both tieredrewards ValidatorEvents and distribution
// ValidatorSlashEvents, then runs a signed MsgClaimTierRewards tx through
// the full AnteHandler + msg handler pipeline and returns gas consumed.
func (s *KeeperSuite) runClaimRewardsGasBenchmark(n, numSlashes int) uint64 {
	s.setupTier(1)
	vals, bondDenom := s.getStakingData()
	valAddr := sdk.MustValAddressFromBech32(vals[0].GetOperator())
	s.setValidatorCommission(valAddr, sdkmath.LegacyZeroDec())

	// Create owner with a signing key and register it with the AccountKeeper
	// so signature verification in the AnteHandler can resolve its pubkey
	// and account number.
	privKey := secp256k1.GenPrivKey()
	ownerAddr := sdk.AccAddress(privKey.PubKey().Address())
	acc := s.app.AccountKeeper.NewAccountWithAddress(s.ctx, ownerAddr)
	s.app.AccountKeeper.SetAccount(s.ctx, acc)
	accNum := acc.GetAccountNumber()

	// Fund owner with enough to lock n positions.
	lockAmount := sdkmath.NewInt(sdk.DefaultPowerReduction.Int64())
	totalLock := lockAmount.MulRaw(int64(n))
	s.Require().NoError(banktestutil.FundAccount(s.ctx, s.app.BankKeeper, ownerAddr,
		sdk.NewCoins(sdk.NewCoin(bondDenom, totalLock))))

	// Create n delegated positions on the same validator.
	msgServer := keeper.NewMsgServerImpl(s.keeper)
	positionIds := make([]uint64, 0, n)
	for range n {
		resp, err := msgServer.LockTier(s.ctx, &types.MsgLockTier{
			Owner:            ownerAddr.String(),
			Id:               1,
			Amount:           lockAmount,
			ValidatorAddress: valAddr.String(),
		})
		s.Require().NoError(err)
		positionIds = append(positionIds, resp.PositionId)
	}

	// Seed validator history by slashing numSlashes times. Each slash:
	//  - fires BeforeValidatorSlashed, appending a ValidatorEvent that
	//    every position's processEventsAndClaimBonus must walk.
	//  - creates a ValidatorSlashEvent in distribution that every
	//    position's WithdrawDelegationRewards iterates.
	// Use a tiny fraction so the validator stays bonded and positions
	// retain non-zero amount across all slashes.
	slashFraction := sdkmath.LegacyNewDecWithPrec(1, 4) // 0.0001 = 1bp
	for range numSlashes {
		s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
		s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Hour))
		s.slashValidatorDirect(valAddr, slashFraction)
	}

	// Advance a block and accrue rewards so both base and bonus paths execute
	// (including the bank SendCoins on the bonus branch).
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(24 * time.Hour))
	s.allocateRewardsToValidator(valAddr, sdkmath.NewInt(1_000_000), bondDenom)
	s.fundRewardsPool(sdkmath.NewInt(1_000_000_000), bondDenom)

	// Build a signed MsgClaimTierRewards tx with a deterministic memo-less
	// layout so per-run variance is minimal.
	msg := &types.MsgClaimTierRewards{
		Owner:       ownerAddr.String(),
		PositionIds: positionIds,
	}
	txCfg := s.app.TxConfig()
	tx, err := simtestutil.GenSignedMockTx(
		rand.New(rand.NewSource(1)),
		txCfg,
		[]sdk.Msg{msg},
		sdk.NewCoins(sdk.NewCoin(bondDenom, sdkmath.ZeroInt())),
		uint64(1_000_000_000), // large tx gas limit so the tx-level meter isn't the bottleneck we're measuring against
		testutil.ChainID,
		[]uint64{accNum},
		[]uint64{0},
		privKey,
	)
	s.Require().NoError(err)

	// Drive the real AnteHandler. SetUpContextDecorator (first in the chain)
	// replaces ctx.GasMeter with a tx-scoped meter sized to tx.GasLimit, and
	// subsequent decorators charge against it (tx size, sig verify, fee, etc.).
	// Explicitly re-set the chain id on the ctx so sig verification resolves
	// the same id used when signing above.
	anteCtx := s.ctx.WithChainID(testutil.ChainID)
	anteHandler := s.app.BaseApp.AnteHandler()
	s.Require().NotNil(anteHandler, "app must have an AnteHandler configured")

	postAnteCtx, err := anteHandler(anteCtx, tx, false)
	s.Require().NoError(err, "ante handler failed")

	// Run the msg handler against the post-ante context. The gas meter
	// attached to postAnteCtx continues to accumulate gas, matching what
	// baseapp's runTx would report as GasUsed.
	_, err = msgServer.ClaimTierRewards(postAnteCtx, msg)
	s.Require().NoError(err, "ClaimTierRewards msg handler failed")

	return postAnteCtx.GasMeter().GasConsumed()
}
