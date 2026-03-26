# Integration Test Review: `test_tieredrewards.py`

Review of `integration_tests/test_tieredrewards.py` against the `x/tieredrewards` module source.
All 32 tests currently pass. Findings are areas for improvement.

---

## Bugs (Wrong Assertions)

| Test | Line | Issue |
|------|------|-------|
| `test_full_exit_flow` | 741 | Bare `except Exception:` catches all errors including network/infra failures. Should catch only `requests.HTTPError` and assert HTTP 404 status code. |
| `test_full_exit_flow` | 732 | `balance_after > balance_before` passes if even 1 token is returned. Should assert `balance_after >= balance_before + amount - gas_allowance`. |
| `test_bonus_stops_after_exit_unlock` | 834 | **Structurally incorrect**: uses an undelegated position which earns zero bonus regardless of exit status. The "bonus stops after `exit_unlock_at`" invariant is never actually exercised. Fix: use a delegated position, verify nonzero bonus before `exit_unlock_at`, then assert zero after. |
| `test_claim_rewards_delegated` | 819 | `balance_after >= balance_before` does not verify actual reward disbursement. Should be `>`. Return value of `find_log_event_attrs` for `EventTierRewardsClaimed` is ignored. |
| `test_fund_tier_pool` | 762 | `balance_after >= 0` is trivially true. Should assert `balance_after >= balance_before + fund_amount`. |
| `test_estimate_rewards_query` | 888 | `"base_rewards" in est or "bonus_rewards" in est` passes even if both are empty lists. Should check both fields exist and at least one has a nonzero amount after waiting blocks. |
| `test_voting_power_after_undelegate` | 1120+ | Never calls `_query_voting_power` after undelegation — only checks the position's `validator` field in the store. The voting power decrease is never actually asserted. |

---

## Reliability Concerns (Fragile Patterns)

### `positions[-1]` lookup

Tests throughout the file identify a target position by taking `positions[-1]` (or filtering and taking the last element) from all positions returned for an owner. Because the cluster is module-scoped, positions accumulate across tests. If test execution order changes, or an earlier test creates an extra position for the same account, the wrong position may be selected.

**Affected tests:** `test_lock_tier_without_validator`, `test_tier_delegate`, `test_tier_undelegate_requires_exit_triggered`, `test_tier_undelegate_after_exit_trigger`, `test_tier_redelegate`, `test_tier_redelegate_when_exiting`, `test_add_to_position`, `test_full_exit_flow`, Group H tests.

**Fix:** Use the `position_id` field from `MsgLockTierResponse` (returned in the tx `msg_responses`) instead of post-hoc position lookup.

### Hardcoded `time.sleep(11)` for unbonding

`test_full_exit_flow` uses `_time.sleep(11)` to wait for unbonding. This is fragile if the genesis unbonding time is changed.

**Fix:** Parse the `completion_time` from the `MsgTierUndelegateResponse` event and use `wait_for_block_time` with that timestamp (same pattern used for `exit_unlock_at`).

### State dependency in `test_add_to_position_while_exiting`

The test looks for an exiting position from `test_lock_tier_with_immediate_exit` (Group B). The conditional fallback creates one if not found, but this makes the test harder to reason about in isolation.

---

## Missing Coverage

### Access Control

| Missing Test | Error | Description |
|-------------|-------|-------------|
| Non-authority rejection | `ErrInvalidSigner` | Submit `MsgAddTier`, `MsgUpdateTier`, `MsgDeleteTier`, or `MsgUpdateParams` directly (not via governance) and verify rejection. This is the most fundamental access-control test for governance-gated messages. |

### Withdraw Error Paths (`ValidateWithdrawFromTier`)

| Missing Test | Error | Description |
|-------------|-------|-------------|
| Withdraw with no exit triggered | `ErrPositionNotReadyToWithdraw` | Attempt to withdraw a fresh (non-exiting) position. |
| Withdraw while still delegated | `ErrPositionStillDelegated` | Trigger exit, wait past `exit_unlock_at`, but do not undelegate — then attempt withdraw. This is the most complex path in `ValidateWithdrawFromTier`. |

### Other Error Paths

| Missing Test | Error | Description |
|-------------|-------|-------------|
| Lock into `close_only` tier | `ErrTierIsCloseOnly` | Set a tier to `close_only` via governance, then attempt to lock tokens into it. |
| `AddTier` with duplicate ID | `ErrTierAlreadyExists` | Submit `MsgAddTier` with an ID that already exists. |
| `DeleteTier` with active positions | `ErrTierHasActivePositions` | Attempt to delete a tier that still has active positions. |
| `TierRedelegate` to same validator | `ErrRedelegationToSameValidator` | `ValidateRedelegatePosition` checks `pos.Validator == dstValidator`. |
| `TierDelegate` on delegated position | `ErrPositionAlreadyDelegated` | `ValidateDelegatePosition` checks `pos.IsDelegated()`. |
| `TierUndelegate` on undelegated position | `ErrPositionNotDelegated` | `ValidateUndelegatePosition` checks `!pos.IsDelegated()`. |
| Double `trigger-exit` | `ErrPositionExiting` | Call `trigger-exit` twice on the same position. `ValidateTriggerExit` checks `pos.HasTriggeredExit()`. |
| Lock into non-existent tier | `ErrTierNotFound` | Attempt to lock into tier ID 99 (or any ID not in genesis/governance state). |
| `CommitDelegationToTier` with active incoming redelegation | `ErrActiveRedelegation` | Commit a delegation that has an active incoming redelegation. |
| `AddToTierPosition` on `close_only` tier | `ErrTierIsCloseOnly` | A position can exist in a tier that is later set to `close_only`; adding to it should be rejected. |

### Side Effects Not Verified

| Missing Test | Description |
|-------------|-------------|
| `MsgFundTierPool` tx | The `fund-tier-pool` CLI subcommand (autocli.go) is never invoked. Current test uses `cluster.transfer` (bank send). `EventTierPoolFunded` is never asserted. |
| Reward claim on `AddToTierPosition` | `msg_server.go` calls `ClaimAndRefreshPosition` before adding to a delegated position. The owner's balance should increase by the pending reward amount. Never verified. |
| Reward claim on `TierRedelegate` | Same: `TierRedelegate` calls `ClaimAndRefreshPosition` before moving the delegation. Side-effect reward disbursement is not asserted. |
| `CommitDelegationToTier` reduces original staking delegation | After committing, the delegator's x/staking delegation to the same validator should decrease by `commit_amount`. The test does not read the staking delegation before/after. |

### Query Coverage

| Missing Test | Description |
|-------------|-------------|
| `AllTierPositions` (paginated) | The `/chainmain/tieredrewards/v1/positions` endpoint with pagination (`next_key` advancement) is never exercised. |
| Exiting+undelegated voting power = 0 | `test_voting_power_undelegated_position` tests non-exiting undelegated. The exiting+undelegated path (exit triggered, then undelegated) is not verified to return 0. |

### Governance / Tally

| Missing Test | Description |
|-------------|-------------|
| Custom tally function exercised | `NewCalculateVoteResultsAndVotingPowerFn` in `gov_tally.go` adds tier position power to staking power during vote counting. No test submits a governance proposal, votes from an account with a tier position, and verifies the outcome reflects the boosted power. |
| `UpdateTier` effect on subsequent `TriggerExit` | After `MsgUpdateTier` changes `ExitDuration`, subsequent `TriggerExit` calls on existing positions should use the new duration. |

---

## Summary by Priority

### High Priority (bugs or test correctness)

1. Fix `test_full_exit_flow` bare `except Exception` → catch `requests.HTTPError`, assert 404
2. Rewrite `test_bonus_stops_after_exit_unlock` with a delegated position
3. Strengthen `test_claim_rewards_delegated` (`>` not `>=`; assert `EventTierRewardsClaimed` attributes)
4. Fix `test_voting_power_after_undelegate` to call `_query_voting_power` and assert the drop
5. Fix `test_fund_tier_pool` balance assertion to `>= balance_before + fund_amount`

### Medium Priority (missing important behaviors)

6. Add test: `ErrPositionStillDelegated` (withdraw while delegated after exit_unlock_at)
7. Add test: non-authority rejection for governance-gated messages (`ErrInvalidSigner`)
8. Add test: `DeleteTier` with active positions (`ErrTierHasActivePositions`)
9. Add test: `TierDelegate` on already-delegated position (`ErrPositionAlreadyDelegated`)
10. Add test: `ErrTierIsCloseOnly` (lock into close_only tier)
11. Add test: `MsgFundTierPool` tx and `EventTierPoolFunded` event

### Lower Priority (reliability / completeness)

12. Replace `positions[-1]` lookups with `position_id` from `MsgLockTierResponse`
13. Replace `time.sleep(11)` with `wait_for_block_time` using `completion_time` from event
14. Add test: implicit reward claim side effect on `AddToTierPosition` and `TierRedelegate`
15. Add test: `CommitDelegationToTier` staking delegation reduction
16. Add test: `AllTierPositions` paginated query
17. Add test: exiting+undelegated position → voting power = 0
18. Add test: custom governance tally with tier position power
