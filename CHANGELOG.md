# Changelog

## UNRELEASED

- [#927](https://github.com/crypto-org-chain/chain-main/pull/927) Integrate versiondb.

*February 20, 2023*

## v4.2.3
Cosmos SDK was upgraded to v0.46.10 and binaries are built with go 1.20.1.

### Improvements
- [925](https://github.com/crypto-org-chain/chain-main/issues/925) Cosmos SDK was upgraded to v0.46.10 

*January 3, 2023*

## v4.2.2
The ibc-go was upgraded to v5.2.0

### Improvements
- [912](https://github.com/crypto-org-chain/chain-main/issues/912) ibc-go was upgraded to v5.2.0

*December 14, 2022*

## v4.2.1
The Cosmos SDK was upgraded to v0.46.7 and binaries are built with go 1.19.4.

### Improvements
- [909](https://github.com/crypto-org-chain/chain-main/issues/909) Cosmos SDK upgraded to v0.46.7

*December 6, 2022*

## v4.2.0
The upgrade plan name was changed to "v4.2.0" (from "v4.0.0") which adds several other
messages to the ICA allowlist (missing staking messages and ICS721).

### Improvements
- [905](https://github.com/crypto-org-chain/chain-main/pull/905) ics23 was bumped to the official 0.9 release.

*December 1, 2022*

## v4.1.3

A bug fix and upgrade to Cosmos SDK to the latest v0.46.x revision and Tendermint to 0.34.24.

### Bug Fixes
- [903](https://github.com/crypto-org-chain/chain-main/pull/903) Fixed denomID and tokenID splitting logic when denom has ibc

### Improvements
- [904](https://github.com/crypto-org-chain/chain-main/pull/904) Cosmos SDK to the latest v0.46.x revision and Tendermint to 0.34.24.

*November 21, 2022*

## v4.1.2

Upgrade to Cosmos SDK to v0.46.6.

### Improvements
- [899](https://github.com/crypto-org-chain/chain-main/pull/899) upgraded Cosmos SDK to v0.46.6.


*November 18, 2022*

## v4.1.1

Upgrade to Cosmos SDK to v0.46.5 and Tendermint to v0.34.23.

### Improvements
- [897](https://github.com/crypto-org-chain/chain-main/pull/897) upgraded Cosmos SDK to v0.46.5 and Tendermint to v0.34.23.

*November 11, 2022*

## v4.1.0

Upgrade to Cosmos SDK to v0.46.5+ and ibc-go to 5.1.

### Improvements
- [896](https://github.com/crypto-org-chain/chain-main/pull/896) upgraded Cosmos SDK to v0.46.5+ and ibc-go to 5.1

*November 2, 2022*

## v4.0.1

A small fix on top of `v4.0.0` and upgrade to Cosmos SDK v0.46.4.

### Improvements
- [890](https://github.com/crypto-org-chain/chain-main/pull/890) added IAVL configuration flags

*October 31, 2022*

## v4.0.0

This is the release of Crypto.org Chain's `v4.0.0`. It contains following changes (when compared with `v3`):

## Added

- Support for interchain accounts. #730 #732 #733
- IBC Fee middleware for incentivizing relayers. #763 #834 
- Add support for NFT transfer via IBC. #860 
- Set default `min_commission_rate` to `5%`. #845 

## Changed

- Upgrade `cosmos-sdk` to `v0.46.3+` and `ibc-go` to `v5.0.1`. #850 #869 #884
- Changed go version to `1.19`. #884
- Change the default priority mechanism to be based on gas price
- Add `url` in `x/nft`'s `Denom`.  #860 

## Deprecated

- Marked `x/supply` module, including its CLI and gRPC, as deprecated. #724

## Security

- Security fixes for Swagger UI. #704

*March 1, 2022*

## v3.3.4

This version is identical to v3.3.3, but 

- Bump `cosmos-sdk`, `ibc-go` and `tendermint` to latest minor versions. #711
- Upgrade Swagger UI version for security fixes. #704

*December 6, 2021*

## v3.3.3
This version is identical to v3.3.2, but 
Cosmos SDK was upgraded to 0.44.5 and Tendermint was upgraded to v0.34.15
(the previous versions contain potential concurrency-related bugs).


*November 29, 2021*

## v3.3.2
This version is identical to v3.3.1, but it's compiled with Go 1.17 and the Cosmos SDK was updated to 0.44.4
which contains a fix for the vesting account migrations (the workaround was removed).


*November 25, 2021*

## v3.3.1
This version is identical to v3.3.0, but its bundled swagger-ui was updated to a newer version
(the previous version contained a potential reflective XSS vulnerability) and a workaround
for vesting account migrations was added in the upgrade handler.

### Bug Fixes
- [679](https://github.com/crypto-org-chain/chain-main/pull/679) workaround for vesting account migrations in the upgrade handler

### Improvements
- [678](https://github.com/crypto-org-chain/chain-main/pull/678) swagger ui updated to 4.1.0


*November 10, 2021*

## v3.3.0
This version is identical to v3.2.0, but ibc-go was upgraded to 2.0.0, Cosmos SDK to 0.44.3,
and Tendermint was upgraded to v0.34.14.

### Improvements
- [665](https://github.com/crypto-org-chain/chain-main/pull/665) default gRPC config listening on 127.0.0.1

*September 30, 2021*

## v3.2.0
This version is identical to the v3.1.1, but did the following dependency upgrades:
- ibc-go was upgraded to 1.2.0;
- tendermint was upgraded to v0.34.13;
- Cosmos SDK was upgraded to 0.44.1.

The upgraded Cosmos SDK version contains a fix for the upgrade non-determinism issue
that was discovered during upgrade testing.

*WARNING*: DO NOT upgrade to this binary yet; instructions are going to be published later
on https://crypto.org/docs/getting-started/upgrade_guide.html .

*September 7, 2021*

## v3.1.1
This version is identical to the v3.1.0, but updated the ibc-go dependency to 1.1.0.
*WARNING*: DO NOT upgrade to this binary yet; instructions are going to be published later
on https://crypto.org/docs/getting-started/upgrade_guide.html .


*September 2, 2021*

## v3.1.0
This version is identical to the v3.0.1, but updated the Cosmos SDK dependency to 0.44.0 which contains a consensus-breaking security patch.
*WARNING*: DO NOT upgrade to this binary yet; instructions are going to be published later
on https://crypto.org/docs/getting-started/upgrade_guide.html .

*August 26, 2021*

## v3.0.1
This version is identical to the v3.0.0, but updated the IBC dependency to 1.0.1 which contains a security patch.
*WARNING*: DO NOT upgrade to this binary yet; instructions are going to be published later
on https://crypto.org/docs/getting-started/upgrade_guide.html .

*August 23, 2021*

## v3.0.0
This version is meant for the "Draco II" mainnet upgrade to enable easier native token issuance via IBC solo machines. It is based on IBC 1.0 and Cosmos SDK 0.43.0 which 
contain several new features (such as the feegrant and authz modules), bug fixes (such as the vesting account bug fix)
and breaking changes. For more details, please see the [Cosmos SDK](https://github.com/cosmos/cosmos-sdk/releases/tag/v0.43.0)
and [ibc-go](https://github.com/cosmos/ibc-go/releases/tag/v1.0.0) release notes.
In addition to that, it also uses Tendermint v0.34.12.
*WARNING*: DO NOT upgrade to this binary yet; instructions are going to be published later
on https://crypto.org/docs/getting-started/upgrade_guide.html .

*August 6, 2021*

## v2.1.2
This version is based on Cosmos SDK 0.42.9 which, among other changes, contains a fix for the non-deterministic app hash mismatch
issues after node restarts or state sync.

*July 12, 2021*

## v2.1.1
This version is based on Cosmos SDK 0.42.7 which, among other changes, contains additional state sync-related fixes.

*June 22, 2021*

## v2.1.0
This version is based on Cosmos SDK 0.42.6 and Tendermint 0.34.11 which, among other changes, contain state sync-related fixes.

### Features
- [565](https://github.com/crypto-org-chain/chain-main/pull/565) NFT module query denom by name

*May 18, 2021*

## v2.0.1
This released version is the same as v2.0.0, but contains an upgrade-critical fix.
NOTE that the upgrade plan name is still expected to be "v2.0.0".
*WARNING*: please follow the upgrade instructions described here: https://crypto.org/docs/getting-started/upgrade_guide.html#the-canis-major-upgrade-guide-v1-to-v2-0-0

### Bug Fixes
- [552](https://github.com/crypto-org-chain/chain-main/pull/552) NFT module store initialized after upgrade

*May 14, 2021*

## v2.0.0
This pre-release version contains the scope of [ADR-003](https://github.com/crypto-org-chain/chain-main/blob/master/doc/architecture/adr-003.md) for the mainnet upgrade. It also upgrades to the latest version of Tendermint (0.34.10).
*WARNING*: please follow the upgrade instructions described here: https://crypto.org/docs/getting-started/upgrade_guide.html#the-canis-major-upgrade-guide-v1-to-v2-0-0

### Breaking changes
- [516](https://github.com/crypto-org-chain/chain-main/pull/516) NFT module added
### Features
- [445](https://github.com/crypto-org-chain/chain-main/pull/445) aggregated tx history query command
- [438](https://github.com/crypto-org-chain/chain-main/pull/438) multi-sign batch command exposed
### Improvements
- [536](https://github.com/crypto-org-chain/chain-main/pull/536) support for Arm64 (Linux and macOS)
- [494](https://github.com/crypto-org-chain/chain-main/pull/494) default timeout_commit adjusted
- [533](https://github.com/crypto-org-chain/chain-main/pull/533) Swagger UI contains chain-main API
### Bug Fixes
- [441](https://github.com/crypto-org-chain/chain-main/pull/441) fix for keyring on Linux

*April 9, 2021*

## v1.2.1
A version based on the upstream release of Cosmos SDK 0.42.4.

*March 26, 2021*

## v1.2.0
A version based on the upstream release of Cosmos SDK 0.42.3.
### Bug Fixes
* [423](https://github.com/crypto-org-chain/chain-main/pull/423) duplicate commands removed

*March 11, 2021*

## v1.1.0
A version based on the upstream release of Cosmos SDK 0.42.1.
### Bug Fixes
* [411](https://github.com/crypto-com/chain-main/pull/411) bank keeper correctly initialized

*March 9, 2021*
## v1.0.1
A version based on the upstream release of Cosmos SDK 0.42.0.
(Note that the SDK 0.42.0 release is nearly identical to the patched SDK fork 0.41.4 that was used in 1.0.0.)

*March 4, 2021*
## v1.0.0
A final initial released version based on a patched fork of Cosmos SDK 0.41.4
(the patch is identical to the 0.8.1 to 0.9.1 releases, i.e. it fixes the bech32 address lookup issue in v0.40/v0.41 SDK releases).

### Breaking changes
* initial modules and custom protobuf were named with "v1"

*February 4, 2021*
## v0.9.1
A version based on a patched fork of Cosmos SDK 0.41.0.

*January 28, 2021*
## v0.9.0
A version based on the release of Cosmos SDK 0.41.0.

*January 20, 2021*
## v0.8.1
A version based on the patched release of Cosmos SDK 0.40.1 that includes security fixes in Tendermint 0.40.3.

*January 11, 2021*
## v0.8.0
A version based on the final release of Cosmos SDK 0.40.
### Improvements
* [347](https://github.com/crypto-com/chain-main/pull/347) keyring options help extended
* [346](https://github.com/crypto-com/chain-main/pull/346) validation of command-line signed integer arguments
* [345](https://github.com/crypto-com/chain-main/pull/345) legacy code removed

*December 17, 2020*
## v0.8.0-rc0 
A version based on rc5 of Cosmos SDK 0.40 release candidate.
### Breaking changes
* many backwards incompatible changes to rc0 of Cosmos SDK 0.40
### Features
* supply querying
* IBC module added
### Improvements
* using upstream Cosmos SDK denomination conversion utilities


*October 14-November 16, 2020*
Early versions based on rc0 of Cosmos SDK 0.40 release candidate.
## v0.7.0-rc* 
### Breaking changes
### Features
### Improvements
### Bug Fixes
