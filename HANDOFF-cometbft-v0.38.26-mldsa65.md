# Handoff: move chain-main to CometBFT v0.38.26 so it accepts ML-DSA-65 Cronos headers

## Goal

Bump chain-main (Cronos POS) from CometBFT v0.38.22 to v0.38.26, through the crypto-org-chain fork. Then its IBC light client of Cronos keeps updating after Cronos validators rotate to ML-DSA-65 consensus keys. chain-main's own validators stay on ed25519.

## Why

- Cronos v1.9 (crypto-org-chain/cronos#2263, SDK v0.55, CometBFT v0.40) lets validators rotate to ML-DSA-65. Turning the key type on is a later governance step (`validator.pub_key_types`), gated on IBC counterparties being ready.
- The Cronos integration plan (`docs/architecture/cosmos-sdk-v0.55-migration-plan.md` §11.4) lists the live counterparties. None of them can verify ML-DSA-65 headers today. chain-main (0.38.22) is the one we control, so it goes first.
- ibc-go v10.5.1 `07-tendermint` decodes headers with `cmttypes.ValidatorSetFromProto` (`header.go:75`, `misbehaviour_handle.go:131`). On v0.38.22 that rejects the key, so `MsgUpdateClient` fails and the Cronos client expires. v0.38.26 decodes and verifies ML-DSA-65 (cometbft/cometbft#6028). No ibc-go or SDK change is needed.
- v0.39.x has no ML-DSA-65 support (checked at v0.39.4). Stay on the v0.38 line.

## State checked on 2026-11-18

- chain-main `master` (73b1c18) and `release/v8` (cfdc545, v8.0.1) both use:
  - `cometbft v0.38.22`, replaced by `crypto-org-chain/cometbft v0.0.0-20261007140153-5cbc9c4af87e`;
  - cosmos-sdk fork `v0.53.7-0.20261007140702-74750d6b782e`, ibc-go v10.5.1, Go 1.25.8.
- The pinned cometbft commit is the tip of `crypto-org-chain/cometbft` branch `release/v0.38.x-pos`. That branch is `v0.38.22` plus one fork commit: `5cbc9c4af perf(indexer): paginate tx search results before loading txs (#69)`. v0.38.26 does not contain it.
- Upstream v0.38.22 → v0.38.26 is 29 commits. Notable changes:
  - v0.38.23: nil vote handling, light witness comparison, ABCI socket panic-recovery fix.
  - v0.38.24: blocksync sender/signature-count checks, mempool `MsgBytesFilter` (heap amplification), `double_sign_check_height = 1` off-by-one, new `EventBusBufferCapacity` config.
  - v0.38.26: ML-DSA-65 key decoding and verification. It adds `cloudflare/circl v1.6.3` as a direct dependency, and the `go` directive stays at 1.22.11.
- Mainnet (`rest.mainnet.crypto.org`, height ≈32.89M): `block.max_bytes = 500000`, `evidence.max_bytes = 150000`, `max_validators = 100`, 100 active validators, `pub_key_types = ["ed25519"]`.

## Block size impact (read before shipping)

v0.38.26 sizes the commit reserve for ML-DSA-65 signatures for **every** validator, whatever key type it uses. `MaxSignatureSize` changes from 64 to 3309, so `MaxCommitSigBytes` goes from 109 to 3355 bytes. Cronos v0.40 does the same.

At mainnet params, with 100 validators:

| | v0.38.22 | v0.38.26 |
|---|---|---|
| Commit reserve (`MaxCommitBytes(100)`) | ~11.3 KB | ~335.9 KB |
| Max tx data per block, no evidence | ~488 KB | ~163 KB |
| Max tx data per block, full 150 KB of evidence | ~338 KB | ~13 KB |

- This only affects the proposer (`state/execution.go:129`) and the mempool size pre-check (`state/tx_filter.go:15`). Block validity doesn't change, so mixed versions don't fork over it.
- Panic thresholds at `max_bytes = 500000`: `MaxDataBytes` goes negative above 104 validators when a block carries the full evidence budget, and `MaxDataBytesNoEvidence` above 148. Today's limit of 100 is safe. Raising `max_validators` above 104 without raising `max_bytes` is not.
- A Cronos `MsgUpdateClient` with ML-DSA-65 validators is about 154 KB (plan §11.4 estimate for 29 validators). That barely fits under the 163 KB pre-check limit, and doesn't fit in a block that carries evidence.
- **Recommendation:** raise `block.max_bytes` through a governance proposal, for example to 850000, which restores ~500 KB of tx space. Do this before or with the binary rollout. First check how full blocks get today; recent blocks are likely far below 163 KB.

## Plan

1. **CometBFT fork.** In crypto-org-chain/cometbft, branch from upstream tag `v0.38.26`, for example `release/v0.38.x-pos-26`, or reset `release/v0.38.x-pos` if the team prefers. Cherry-pick `5cbc9c4af` (#69) with `git cherry-pick -x`, keeping author and date. Run `go test ./state/... ./types/... ./rpc/... ./crypto/...`, open a PR, and merge it with a merge commit or rebase, not squash.
2. **chain-main.** On `master`:
   - point the `replace github.com/cometbft/cometbft` line at the new fork commit, and bump the `require` line from `v0.38.22` to `v0.38.26`;
   - run `go mod tidy`, regenerate `gomod2nix.toml` (`gomod2nix`), then `go build ./...`, the unit tests, and `nix build`;
   - check `vendor/` is not committed; it is untracked locally.
3. **Test.** Add a Go test or an integration test: build a 07-tendermint header whose validator set holds an ML-DSA-65 key, and check that `MsgUpdateClient` succeeds on chain-main. ethermint has an example: crypto-org-chain/ethermint#1146, the ML-DSA-65 header test through `07-tendermint`. Also run the existing IBC integration tests.
4. **CHANGELOG.** Add one entry under `## UNRELEASED` → `### Improvements`, in the repo's format: `- [#NNNN](https://github.com/crypto-org-chain/chain-main/pull/NNNN) <PR title>.` Mention the block-size change in the PR body and the release notes.
5. **Release.** Backport to `release/v8` and cut a release, for example v8.1.0.

## Rollout: this is consensus-relevant, but only later

- Before any Cronos validator holds an ML-DSA-65 key, v0.38.22 and v0.38.26 nodes behave the same, so a rolling binary upgrade is safe.
- Once a Cronos header carries an ML-DSA-65 key, a v0.38.22 node fails `MsgUpdateClient` and a v0.38.26 node accepts it. Different tx results mean an **app-hash fork**.
- So every chain-main validator must run the new binary **before** Cronos governance adds `ml_dsa_65` to `pub_key_types`. Two options; the team decides:
  - **A (simpler):** patch release, no upgrade height. Confirm adoption across the validator set (node versions, or a deadline with the validators) before the Cronos proposal.
  - **B (enforced):** ship it in the next chain-main software upgrade, so the switch happens at a fixed height.
- Raise `block.max_bytes` before the binary reaches most proposers, or tx space per block drops by about two thirds.

## Relayers

- **Hermes:** use the crypto-org-chain/hermes fork (crypto-org-chain/hermes#11, merged at `cd4664d8`; ML-DSA-65 via a tendermint-rs v0.40 backport). Hermes needs it to read Cronos headers, whatever chain-main runs. Upstream Hermes can't decode ML-DSA keys yet (checked at v1.13.3; tendermint-rs support is in open PRs cometbft/tendermint-rs#1532/#1533). The fork patches tendermint-rs from a personal fork rev; move it to an org fork or upstream before mainnet. For chain-main, `compat_mode` can stay unset: Hermes detects `0.38` from the node version, or you can set `compat_mode = '0.38'` explicitly. Hermes has no `0.40` mode. Cronos (CometBFT v0.40) must set `compat_mode = '0.38'`, otherwise Hermes falls back to `0.37`.
- **rly:** use the crypto-org-chain/relayer fork (crypto-org-chain/relayer#22, merged at `35da7da1`, CometBFT v0.38.26).
- Cronos pins and tests both forks (crypto-org-chain/cronos#2263).

## Cronos follow-ups after the chain-main release

- `nix/sources.json` still pins `chain-main` at `release/v7` (`d3ef39af`, CometBFT 0.38.17). Re-pin it to the new release.
- `integration_tests/test_ibc_mldsa65.py` (Hermes and rly) uses a Cronos↔Cronos pair, because chain-main couldn't be the counterparty. Add a Cronos↔chain-main case for both relayers.
- Update the Cronos plan §11.4 rows once chain-main mainnet runs v0.38.26: "IBC counterparties" (version) and "Counterparty block space" (the new `block.max_bytes`).
