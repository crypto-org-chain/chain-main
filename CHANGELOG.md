# Changelog

*unreleased*

*January 27, 2022*

## v1.2.2
  This release is identical to v1.1.1, but is built with Go 1.17 and supports Arm64 binaries as well as a RocksDB database backend.
### Improvements
* support release for Arm64 (Linux and macOS)
* support rocksdb in makefile
### Bug Fixes
* resolve go module name v1 issue

*Jan 19, 2022*

## v2.0.0

### Breaking changes

- [381](https://github.com/crypto-org-chain/chain-main/pull/381) Add subscription module

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
