# Tiered Rewards: ADR-006 Gap Analysis & Security Review

**PR:** [#1249](https://github.com/crypto-org-chain/chain-main/pull/1249)
**Branch:** `feat/tokenomic`
**Date:** 2026-03-24 (updated from initial 2026-03-17 review against PR #1242)

---

## Executive Summary

The implementation is **substantially complete** with the core tier lock, delegation, exit, rewards, slashing, governance, and genesis mechanisms all functional. Since the initial review (PR #1242), many critical gaps have been addressed: `MsgWithdrawFromTier`, `MsgFundTierPool`, genesis export/import, all queries, and governance tally are now implemented.

Remaining issues: **3 critical deviations** from the ADR spec requiring design decisions, **1 high-severity security finding**, and several medium items.

---

## Part 1: ADR-006 Implementation Gap Analysis

### Legend

- ✅ Implemented (matches ADR)
- ⚠️ Partially implemented / deviation
- ❌ Not implemented
- ➕ Additional (beyond ADR)

---

### 1. Messages (ADR §5)

| # | Message | Status | Notes |
|---|---------|--------|-------|
| 1 | `MsgLockTier` (with optional validator + trigger_exit_immediately) | ✅ | All fields present; lock + optional delegate + optional exit in one step |
| 2 | `MsgCommitDelegationToTier` (partial/full, TransferDelegation) | ✅ | Partial commit supported; TransferDelegation via Unbond+Delegate; also supports `trigger_exit_immediately` (beyond ADR) |
| 3 | `MsgAddToTierPosition` (reject when exiting) | ✅ | Option B (settle bonus before add) chosen |
| 4 | `MsgTierDelegate` | ⚠️ **DEVIATION-1** | Rejects exiting positions — breaks the ADR §10 flow "lock + trigger_exit_immediately + delegate later" |
| 5 | `MsgTierUndelegate` | ⚠️ **DEVIATION-2** | Requires exit commitment **elapsed** (not just triggered). ADR says only `ExitTriggeredAt != 0` required |
| 6 | `MsgTierRedelegate` | ✅ | Full shares; also rejects exiting positions (additional constraint) |
| 7 | `MsgTriggerExitFromTier` | ✅ | Sets ExitTriggeredAt + ExitUnlockTime correctly |
| 8 | `MsgWithdrawFromTier` (claim) | ✅ | Two-phase: requires exit elapsed + not delegated |
| 9 | `MsgClaimTierRewards` (ADR: MsgWithdrawTierRewards) | ✅ | Name differs; base via cumulative ratio batching |
| 10 | `MsgFundTierPool` | ⚠️ | Permissionless (any account can fund); ADR recommends authority restriction |
| 11 | `MsgClaimExpiredTier` (optional) | ❌ | Covered by MsgWithdrawFromTier |
| 12 | `MsgTransferTierPosition` (optional) | ❌ | Explicitly optional in ADR |
| - | `MsgAddTier` / `MsgUpdateTier` / `MsgDeleteTier` / `MsgUpdateParams` | ➕ | Authority-gated tier CRUD; not in ADR but needed for governance |

---

### 2. State Model (ADR §4)

#### 2.1 Tier Definition

| ADR Field | Implementation | Status |
|-----------|---------------|--------|
| `TierId` (uint32) | `Tier.Id` (uint32) | ✅ |
| `ExitCommitmentDuration` | `Tier.ExitDuration` (Duration) | ✅ |
| `ExitCommitmentDurationInYears` (int64) | — | ❌ Missing (informational only) |
| `BonusApy` (Dec) | `Tier.BonusApy` (LegacyDec) | ✅ |
| `MinLockAmount` (Int) | `Tier.MinLockAmount` (Int) | ✅ |
| — | `Tier.CloseOnly` (bool) | ➕ Prevents new positions on deprecated tier |

#### 2.2 Params

| ADR Field | Implementation | Status |
|-----------|---------------|--------|
| `Tiers []TierDefinition` | Separate `collections.Map[uint32, Tier]` | ⚠️ Stored separately (better design, different structure) |
| `BonusDenoms []string` | — | ❌ Not implemented; bonus always in bond denom |
| — | `TargetBaseRewardsRate` (LegacyDec) | ➕ For BeginBlocker base rewards top-up |

#### 2.3 TierPosition

| ADR Field | Implementation | Status |
|-----------|---------------|--------|
| `PositionId` (uint64) | `Position.Id` | ✅ |
| `Owner` (string) | `Position.Owner` | ✅ |
| `TierId` (uint32) | `Position.TierId` | ✅ |
| `AmountLocked` (Int) | `Position.Amount` | ✅ |
| `CreatedAtHeight` (int64) | `Position.CreatedAtHeight` (uint64) | ✅ |
| `CreatedAtTime` | `Position.CreatedAtTime` | ✅ |
| `ExitTriggeredAt` | `Position.ExitTriggeredAt` | ✅ |
| `ExitUnlockTime` | `Position.ExitUnlockAt` | ✅ |
| `Validator` (string) | `Position.Validator` | ✅ |
| `DelegatedShares` (Dec) | `Position.DelegatedShares` | ✅ |
| `DelegatedAtTime` | — | ❌ Missing (`LastBonusAccrual` serves similar purpose) |
| `LastBonusAccrual` | `Position.LastBonusAccrual` | ✅ |
| — | `Position.BaseRewardsPerShare` (DecCoins) | ➕ For cumulative ratio batching |

#### 2.4 Store Layout

| ADR Store | Implementation | Status |
|-----------|---------------|--------|
| `PositionByID` | `Positions: Map[uint64, Position]` | ✅ |
| `PositionsByOwner` | `PositionsByOwner: KeySet` | ✅ |
| `NextPositionId` | `NextPositionId: Sequence` | ✅ |
| — | `PositionsByTier`, `PositionsByValidator`, `PositionCountByTier` | ➕ |
| — | `ValidatorRewardRatio`, `UnbondingIdToPositionId` | ➕ |

---

### 3. Queries (ADR §6)

| ADR Query | Implementation | Status |
|-----------|---------------|--------|
| `TierParams` | `Params` + `Tiers` (split) | ✅ |
| `TierPoolBalance` | `TierPoolBalance` | ✅ |
| `PositionByID` | `TierPosition` | ✅ |
| `PositionsByOwner(pagination)` | `TierPositionsByOwner` | ⚠️ No pagination |
| `AllTierPositions(pagination)` | `AllTierPositions` | ✅ |
| `EstimateTierBonus` | `EstimateTierRewards` (base + bonus) | ✅ |
| `TierVotingPower` | `TierVotingPower` | ✅ |

---

### 4. Governance (ADR §8)

| ADR Requirement | Implementation | Status |
|----------------|---------------|--------|
| `GetVotingPowerForAddress` | Uses shares-to-tokens conversion (ADR §8.2 allows this) | ⚠️ **DEVIATION-3**: Excludes exiting positions (ADR §8.5 says they should count) |
| `TotalDelegatedVotingPower` | Implemented (informational only) | ✅ |
| Custom gov tally | `WithCustomCalculateVoteResultsAndVotingPowerFn` with DelegatorDeductions | ✅ |

---

### 5. Rewards (ADR §4.5)

| ADR Requirement | Implementation | Status |
|----------------|---------------|--------|
| Base rewards via x/distribution | Cumulative ratio batching (ADR §5.7.1 optimization) | ✅ |
| Bonus: `AmountLocked × BonusApy × duration_years` | Uses `TokensFromShares(DelegatedShares)` instead of `AmountLocked` | ⚠️ More correct post-slash but differs from literal formula |
| `SecondsPerYear` constant | 31,557,600 (365.25 days) | ✅ |
| Cap accrual_end at ExitUnlockTime | Implemented | ✅ |
| Cap bonus to pool balance | Returns `ErrInsufficientBonusPool` (fails entire bonus, not partial) | ⚠️ ADR is self-contradictory; impl matches "fail and defer" interpretation |
| No rewards when not delegated | Enforced | ✅ |
| Settle bonus before AddToPosition (Option B) | ✅ `ClaimAndRefreshPosition` called before adding when delegated | ✅ (Fixed since PR #1242) |

---

### 6. Slashing (ADR §4.6)

| ADR Requirement | Implementation | Status |
|----------------|---------------|--------|
| Update AmountLocked on slash | `BeforeValidatorSlashed` hook | ✅ |
| Update DelegatedShares on redelegation slash | `AfterSlashRedelegation` | ✅ |
| Handle unbonding delegation slash | `AfterSlashUnbondingDelegation` | ✅ |
| Claim rewards before slash | Calls `claimAllRewardsForPositions` first | ✅ |
| `AfterValidatorBeginUnbonding` — settle rewards | Claims all with `forceAccrue=true` | ✅ |
| `AfterValidatorBonded` — reset bonus accrual | Resets `LastBonusAccrual` to block time | ✅ |

---

### 7. Events, Genesis, ABCI

| Item | Status | Notes |
|------|--------|-------|
| Events for all major operations | ✅ | 15+ typed events covering all messages, rewards, slashing |
| Genesis import/export | ✅ | Tiers, positions, params, reward ratios, unbonding mappings, NextPositionId |
| Genesis validation | ✅ | Cross-references positions vs tiers, validates IDs, checks unbonding mappings |
| ABCI | ⚠️ | BeginBlocker for `topUpBaseRewards` (not in ADR; ADR says "no-op or future pool refill") |

---

### 8. Progress Since PR #1242 Review

| Previously Critical Gap | Status |
|------------------------|--------|
| `MsgWithdrawFromTier` (tokens permanently stuck) | ✅ **Fixed** |
| Genesis export/import (state lost on upgrade) | ✅ **Fixed** |
| Query endpoints (module unusable by UIs) | ✅ **Fixed** |
| Governance voting power (tier lockers had no voice) | ✅ **Fixed** |
| `MsgFundTierPool` (no way to fund pool) | ✅ **Fixed** |
| Settle bonus before AddToPosition (Option B) | ✅ **Fixed** |

---

## Part 2: Critical Deviations Requiring Decision

### DEVIATION-1: MsgTierDelegate Rejects Exiting Positions

**File**: `x/tieredrewards/keeper/msg_validate.go` (`ValidateDelegatePosition`)

**ADR says** (§10): "Lock with trigger_exit_immediately but no validator: **Allowed**. Position is created in exiting state. No rewards until the user delegates via MsgTierDelegate; once delegated, bonus accrues at fixed APY until ExitUnlockTime."

**Implementation**: `ValidateDelegatePosition` rejects delegation when `HasTriggeredExit()` is true.

**Impact**: A position created with `trigger_exit_immediately=true` and no validator becomes a **dead-end state** — can never be delegated, can never earn rewards. Tokens locked for the exit duration with zero return.

**Options**:
- A) Remove `HasTriggeredExit()` check from `ValidateDelegatePosition` to match ADR
- B) Reject `MsgLockTier` when `trigger_exit_immediately=true` and no validator is specified
- C) Update ADR to match implementation (document this as intentional)

---

### DEVIATION-2: MsgTierUndelegate Requires Exit Commitment Elapsed

**File**: `x/tieredrewards/keeper/msg_validate.go` (`ValidateUndelegatePosition`)

**ADR says** (§5.4): "require position has triggered exit (`ExitTriggeredAt != 0`)" — undelegation allowed immediately after triggering exit.

**Implementation**: Requires `CompletedExitLockDuration(blockTime)` — the **full exit commitment must have elapsed** before undelegation.

**Impact**: Users must remain delegated for the **entire exit commitment period** (e.g., 5 years) THEN begin SDK unbonding (21-28 days). Total = exit + unbonding. ADR intended: total = max(exit, unbonding) since they could overlap.

**Options**:
- A) Change to `pos.HasTriggeredExit()` to match ADR (allow undelegation during exit period)
- B) Keep stricter behavior and update ADR (forces longer capital lockup but simpler)

---

### DEVIATION-3: Governance Voting Power Excludes Exiting Positions

**File**: `x/tieredrewards/types/position.go` (`IsActiveForGovernance`)

**ADR says** (§8.5): "Exit triggered, still delegated: Position remains delegated until the owner calls MsgTierUndelegate and unbonding completes. **Until then, it still counts for tier voting power.**"

**Implementation**: `IsActiveForGovernance()` returns `IsDelegated() && !HasTriggeredExit()` — exiting positions excluded.

**Impact**: Users who trigger exit lose governance voting power immediately, even though they remain delegated and providing network security.

**Options**:
- A) Change `IsActiveForGovernance()` to `IsDelegated()` to match ADR
- B) Keep current behavior and update ADR (incentivizes staying in tier for governance power)

---

## Part 3: Security Review

### Severity Summary

| Severity | Count |
|----------|-------|
| Critical | 0 |
| High | 1 |
| Medium | 5 |
| Low | 6 |

---

### HIGH

#### H-1: Missing `RegisterInterfaces` / `RegisterLegacyAminoCodec` for Two Messages

**File**: `x/tieredrewards/types/codec.go`

`MsgWithdrawFromTier` and `MsgFundTierPool` are NOT registered in `RegisterInterfaces` or `RegisterLegacyAminoCodec`. These messages will fail to deserialize when wrapped in `MsgExec` (authz grants) or governance proposals.

**Fix**:
```go
// In RegisterInterfaces:
registry.RegisterImplementations((*sdk.Msg)(nil),
    // ... existing ...
    &MsgWithdrawFromTier{},
    &MsgFundTierPool{},
)

// In RegisterLegacyAminoCodec:
legacy.RegisterAminoMsg(cdc, &MsgWithdrawFromTier{}, "chainmain/MsgWithdrawFromTier")
legacy.RegisterAminoMsg(cdc, &MsgFundTierPool{}, "chainmain/MsgFundTierPool")
```

---

### MEDIUM

#### M-1: Unbounded State Growth in `UnbondingIdToPositionId` Mapping

**File**: `x/tieredrewards/keeper/msg_server.go`

Entries created on undelegate/redelegate are never cleaned up. Existing TODO comments acknowledge this. Over the chain's lifetime, this map grows indefinitely.

**Recommendation**: Implement cleanup via EndBlocker or hook callback.

#### M-2: `MsgFundTierPool` Missing `sdk.Msg` Assertion and `Validate()` Method

**File**: `x/tieredrewards/types/msgs.go`

Missing `_ sdk.Msg = &MsgFundTierPool{}` compile-time assertion and a proper `Validate()` method (inconsistent with other messages).

#### M-3: `topUpBaseRewards` Rounding Leaves Dust in Distribution Module

**File**: `x/tieredrewards/keeper/abci.go`

Per-validator allocations are truncated independently. Dust (sum of truncated remainders) has been sent to distribution but never allocated. Accumulates over time.

#### M-4: Base Rewards Payout Not Capped to Module Account Balance

**File**: `x/tieredrewards/keeper/rewards.go`

Unlike bonus rewards (which check pool balance), base reward payouts don't verify module account balance. If rounding diverges, `SendCoinsFromModuleToAccount` could fail.

#### M-5: `MsgAddToTierPosition` Does Not Validate Minimum on Added Amount

**File**: `x/tieredrewards/keeper/msg_validate.go`

`AddToTierPosition` allows adding any positive amount (even 1 unit). Initial `LockTier` enforces `MinLockAmount`, but adds do not. Consistent with ADR ("MsgAddToTierPosition does not require the added amount to meet any minimum") but worth documenting explicitly.

---

### LOW / INFORMATIONAL

| ID | Finding | File |
|----|---------|------|
| L-1 | `BeginBlocker` panics on parameter fetch failures (consistent with SDK conventions) | `keeper/abci.go` |
| L-2 | `slash` reduces `pos.Amount` but not `DelegatedShares` (shares naturally reflect slash via exchange rate) | `keeper/rewards.go` |
| L-3 | Governance tally edge case when module account votes (safely handled via clamping) | `keeper/gov_tally.go` |
| L-4 | `WithdrawFromTier` uses `Positions.Get` directly instead of `GetPosition` wrapper | `keeper/msg_server.go` |
| L-5 | Genesis `ValidateGenesis` does not validate `CumulativeRewardsPerShare` non-negativity | `types/genesis.go` |
| L-6 | `TriggerExit` can be called on undelegated positions (valid flow but potential UX concern) | `keeper/msg_validate.go` |

---

### Positive Security Observations

1. **Strong authorization model** — all messages properly gated with ownership + authority checks
2. **Sound cumulative rewards-per-share pattern** — avoids inter-position double-counting
3. **Comprehensive slashing coverage** — handles all four slash vectors correctly
4. **Correct partial-failure semantics** — failed bonus claims don't advance `LastBonusAccrual`
5. **Transfer delegation guards** — blocks self-transfers, validates bonded status, checks active redelegations
6. **BonusAPY and TargetBaseRewardsRate capped at 100%** — prevents governance parameter exploits
7. **Thorough position validation** — internal consistency enforced on state transitions
8. **Comprehensive genesis validation** — cross-references positions vs tiers, validates IDs
9. **Correct BeginBlocker ordering** — placed before x/distribution for base rewards top-up
10. **`ClaimAndRefreshPosition` pattern** — re-fetches position after claiming, preventing stale state bugs
11. **Governance tally double-counting prevention** — DelegatorDeductions correctly applied per-voter

---

## Part 4: Recommended Action Items

### Before Merge (Blocking)

| # | Item | Severity |
|---|------|----------|
| 1 | **Fix H-1**: Register `MsgWithdrawFromTier` and `MsgFundTierPool` in codec.go | High |
| 2 | **Decide DEVIATION-1, -2, -3**: Are these intentional design changes? Fix impl or update ADR | Design |

### Post-Merge (Track)

| # | Item | Priority |
|---|------|----------|
| 3 | Implement `UnbondingIdToPositionId` cleanup (M-1) | Medium |
| 4 | Add `MsgFundTierPool` to sdk.Msg assertion + `Validate()` (M-2) | Medium |
| 5 | Add pagination to `PositionsByOwner` query | Medium |
| 6 | Document/fix `topUpBaseRewards` rounding dust (M-3) | Low |
| 7 | Cap base reward payout to module account balance (M-4) | Low |
| 8 | Consider adding `BonusDenoms` param for multi-denom flexibility | Low |
| 9 | Consider adding `DelegatedAtTime` field for informational queries | Low |

---

## Appendix: File Reference

| Component | Path |
|-----------|------|
| ADR-006 | `doc/architecture/adr-006.md` |
| Proto definitions | `proto/chainmain/tieredrewards/v1/` |
| Module code | `x/tieredrewards/` |
| Keeper | `x/tieredrewards/keeper/` |
| Types | `x/tieredrewards/types/` |
| App integration | `app/app.go` |
