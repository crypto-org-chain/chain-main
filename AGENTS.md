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

## Go coding style

Follow the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md). Formatting is enforced by `gofumpt` (with extra rules) and `gci` import ordering — always run `make lint-fix` before committing. Import groups, in order: standard library, third party, `cosmossdk.io`, then `github.com/cosmos/cosmos-sdk`.

Key guidelines the linters do not fully enforce:

- **Errors**: wrap with `fmt.Errorf("...: %w", err)`; check with `errors.Is`/`errors.As` (`errorlint` enforces this). Error strings are lowercase and unpunctuated. Error vars are named `errFoo`; error types `FooError`.
- **Don't panic** in library/module code — return errors. Panic only for truly irrecoverable states (e.g. invariant violations in `app` wiring).
- **Type assertions**: always use the comma-ok form (`v, ok := x.(T)`) to avoid panics.
- **Reduce nesting**: handle errors and special cases early with `return`/`continue`; avoid unnecessary `else` after a `return`.
- **Initialize structs with field names**; omit zero-value fields.
- **Enums** using `iota` start at one (or a sentinel) so the zero value is meaningful.
- **Interfaces**: verify compliance at compile time with `var _ Iface = (*Impl)(nil)` where it adds clarity; pass interfaces by value, not pointer.
- **Naming**: keep initialisms consistent-case (`ID`, `URL`, `RPC`); don't shadow built-ins (`len`, `error`, `min`); prefer clear, short names.
- **Avoid mutable globals and `init()` side effects** — prefer dependency injection via the app/keeper constructors.
- **Copy slices and maps at boundaries** when storing or returning caller-supplied references.
- **Tests**: prefer table-driven tests; use `t.Helper()` in test helpers (`thelper` enforces this).
- **Performance**: prefer `strconv` over `fmt` for conversions; pre-size slices/maps with `make(..., cap)` when the size is known.
