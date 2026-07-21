# AGENTS.md

Guidance for AI coding agents working in this repository. Keep this file factual and current — it documents *how the project is built and organized*, not task history.

## Project

Crypto.org Chain (`chain-main`) — a Cosmos SDK based blockchain. The daemon binary is `chain-maind` (entrypoint `./cmd/chain-maind`).

- Go module: `github.com/crypto-org-chain/chain-main/v8` (Go 1.25.x)
- Built on the Cosmos SDK (`cosmossdk.io/*`, `github.com/cosmos/cosmos-sdk`)

## Layout

- `app/` — application wiring: `app.go`, `ante.go`, `encoding.go`, `genesis.go`, `upgrades.go`, `export.go`
- `cmd/chain-maind/` — CLI entrypoint
- `x/` — custom modules: `chainmain`, `inflation`, `nft`, `nft-transfer`, `supply`, `tieredrewards`
- `proto/` — protobuf definitions; `third_party/` vendored protos
- `integration_tests/` — Python (pytest) integration tests, driven via Nix
- `pystarport/` — local devnet tooling
- `nix/`, `flake.nix`, `default.nix` — Nix build/dev environment

## Build / test / lint commands

Run these from the repo root:

- Build: `make build` (outputs to `build/chain-maind`) or `make install`
- Unit tests: `make test`
- Lint: `make lint` (runs `golangci-lint run` + `go mod verify`)
- Lint autofix: `make lint-fix`
- Regenerate protobufs: `make make-proto`
- Nix integration tests: `make nix-integration-test*` targets (e.g. `nix-integration-test-all`)

## Build tags

The Makefile assembles `build_tags` including `netgo`, `ledger` (when `LEDGER_ENABLED=true`, the default; requires gcc), and `libsecp256k1_sdk`. Optional DB backends via `COSMOS_BUILD_OPTIONS` add tags: `rocksdb grocksdb_clean_link`, `badgerdb`, `boltdb`. A `testnet` tag exists for testnet builds. Building with `rocksdb` requires the `rocksdb` pkg-config package on the system.

## Conventions

- Commits/PRs follow Conventional Commits (`fix(x/nft): ...`, `chore(ci): ...`) — see `CONTRIBUTING.md`.
- User-facing changes get a `CHANGELOG.md` entry.
- Go code is linted with `golangci-lint` (config in `.golangci.yml`); Python with flake8/isort (`.flake8`, `.isort.cfg`).
