PACKAGES=$(shell go list ./... | grep -v '/simulation')
PACKAGE_NAME:=github.com/crypto-org-chain/chain-main
GOLANG_CROSS_VERSION  = v1.20.1


VERSION := $(shell echo $(shell git describe --tags 2>/dev/null ) | sed 's/^v//')
TMVERSION := $(shell go list -m github.com/tendermint/tendermint | sed 's:.* ::')
COMMIT := $(shell git log -1 --format='%H')
NETWORK ?= mainnet
COVERAGE ?= coverage.txt
BUILDDIR ?= $(CURDIR)/build
LEDGER_ENABLED ?= true

export GO111MODULE = on

build_tags = netgo
ifeq ($(LEDGER_ENABLED),true)
  ifeq ($(OS),Windows_NT)
    GCCEXE = $(shell where gcc.exe 2> NUL)
    ifeq ($(GCCEXE),)
      $(error gcc.exe not installed for ledger support, please install or set LEDGER_ENABLED=false)
    else
      build_tags += ledger
    endif
  else
    UNAME_S = $(shell uname -s)
    ifeq ($(UNAME_S),OpenBSD)
      $(warning OpenBSD detected, disabling ledger support (https://github.com/cosmos/cosmos-sdk/issues/1988))
    else
      GCC = $(shell command -v gcc 2> /dev/null)
      ifeq ($(GCC),)
        $(error gcc not installed for ledger support, please install or set LEDGER_ENABLED=false)
      else
        build_tags += ledger
      endif
    endif
  endif
endif

ifeq (secp,$(findstring secp,$(COSMOS_BUILD_OPTIONS)))
  build_tags += libsecp256k1_sdk
endif

# DB backend selection
ifeq (cleveldb,$(findstring cleveldb,$(COSMOS_BUILD_OPTIONS)))
  build_tags += gcc
endif
ifeq (badgerdb,$(findstring badgerdb,$(COSMOS_BUILD_OPTIONS)))
  BUILD_TAGS += badgerdb
endif
# handle rocksdb
ifeq (rocksdb,$(findstring rocksdb,$(COSMOS_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += rocksdb
endif
# handle boltdb
ifeq (boltdb,$(findstring boltdb,$(COSMOS_BUILD_OPTIONS)))
  BUILD_TAGS += boltdb
endif

ifeq ($(NETWORK),testnet)
	BUILD_TAGS += testnet
	test_tags += testnet
endif

build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

whitespace :=
whitespace += $(whitespace)
comma := ,
build_tags_comma_sep := $(subst $(whitespace),$(comma),$(build_tags))

# process linker flags

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=crypto-org-chain-chain \
	-X github.com/cosmos/cosmos-sdk/version.AppName=chain-maind \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
	-X github.com/tendermint/tendermint/version.TMCoreSemVer=$(TMVERSION) \
	-X "github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags_comma_sep)"

ifeq (,$(findstring nostrip,$(COSMOS_BUILD_OPTIONS)))
  ldflags += -w -s
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'
# check for nostrip option
ifeq (,$(findstring nostrip,$(COSMOS_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
endif

# Check for debug option
ifeq (debug,$(findstring debug,$(COSMOS_BUILD_OPTIONS)))
  BUILD_FLAGS += -gcflags "all=-N -l"
endif

TEST_FLAGS := -tags "$(test_tags)"

TESTNET_FLAGS ?=

SIMAPP = github.com/crypto-org-chain/chain-main/v4/app
BINDIR ?= ~/go/bin

OS := $(shell uname)

all: download install

download:
	git submodule update --init --recursive

install: check-network go.sum
		go install -mod=readonly $(BUILD_FLAGS) ./cmd/chain-maind

build: check-network go.sum
		go build -mod=readonly $(BUILD_FLAGS) -o $(BUILDDIR)/chain-maind ./cmd/chain-maind

buildwindows: check-network go.sum
		go build -buildmode=exe -mod=readonly $(BUILD_FLAGS) -o $(BUILDDIR)/chain-maind ./cmd/chain-maind


.PHONY: build

build-linux: go.sum
ifeq ($(OS), Linux)
		GOOS=linux GOARCH=amd64 $(MAKE) build
else
		LEDGER_ENABLED=false GOOS=linux GOARCH=amd64 $(MAKE) build
endif

build-mac: go.sum
ifeq ($(OS), Darwin)
		GOOS=darwin GOARCH=amd64 $(MAKE) build
else
		LEDGER_ENABLED=false GOOS=darwin GOARCH=amd64 $(MAKE) build
endif

go.sum: go.mod
		@echo "--> Ensure dependencies have not been modified"
		GO111MODULE=on go mod verify

test: check-network
	@go test $(TEST_FLAGS) -v -mod=readonly $(PACKAGES) -coverprofile=$(COVERAGE) -covermode=atomic
.PHONY: test

# look into .golangci.yml for enabling / disabling linters
lint:
	@echo "--> Running linter"
	@golangci-lint run
	@go mod verify
	@flake8 --show-source --count --statistics
	@find . -name "*.nix" -type f | xargs nixpkgs-fmt --check

# a trick to make all the lint commands execute, return error when at least one fails.
# golangci-lint is run in standalone job in ci
lint-ci:
	@echo "--> Running linter for CI"
	@nix-shell --pure -E "with (import ./nix {}); mkShell { buildInputs = [lint-ci]; }" --run lint-ci

test-sim-nondeterminism: check-network
	@echo "Running non-determinism test..."
	@go test $(TEST_FLAGS) -mod=readonly $(SIMAPP) -run TestAppStateDeterminism -Enabled=true \
		-NumBlocks=100 -BlockSize=200 -Commit=true -Period=0 -v -timeout 24h

test-sim-custom-genesis-fast: check-network
	@echo "Running custom genesis simulation..."
	@echo "By default, ${HOME}/.chain-maind/config/genesis.json will be used."
	@go test $(TEST_FLAGS) -mod=readonly $(SIMAPP) -run TestFullAppSimulation -Genesis=${HOME}/.gaiad/config/genesis.json \
		-Enabled=true -NumBlocks=100 -BlockSize=200 -Commit=true -Seed=99 -Period=5 -v -timeout 24h -ExitOnFail

test-sim-import-export:
	@echo "Running Chain import/export simulation. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 25 5 TestAppImportExport

test-sim-after-import:
	@echo "Running application simulation-after-import. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) -ExitOnFail 50 5 TestAppSimulationAfterImport

###############################################################################
###                                Localnet                                 ###
###############################################################################

build-docker-chainmaindnode:
	$(MAKE) -C check-networks/local

# Run a 4-node testnet locally
localnet-start: build-linux build-docker-chainmaindnode localnet-stop
	@if ! [ -f $(BUILDDIR)/node0/.chain-maind/config/genesis.json ]; \
	then docker run --rm -v $(BUILDDIR):/chain-maind:Z cryptocom/chainmaindnode testnet --v 4 -o . --starting-ip-address 192.168.10.2 $(TESTNET_FLAGS); \
	fi
	BUILDDIR=$(BUILDDIR) docker-compose up -d

# Stop testnet
localnet-stop:
	docker-compose down
	docker check-network prune -f

# local build pystarport
build-pystarport:
	pip install ./pystarport

# Run a local testnet by pystarport
localnet-pystartport: build-pystarport
	pystarport serve

clean:
	rm -rf $(BUILDDIR)/

clean-docker-compose: localnet-stop
	rm -rf $(BUILDDIR)/node* $(BUILDDIR)/gentxs

create-systemd:
	./networks/create-service.sh

make-proto:
	./makeproto.sh
###############################################################################
###                                Nix                                      ###
###############################################################################
# nix installation: https://nixos.org/download.html
nix-integration-test: check-network make-proto
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m 'not upgrade and not ledger and not slow and not ibc and not byzantine and not gov and not grpc and not solomachine'"

nix-integration-test-solomachine: check-network
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m solomachine"

nix-integration-test-upgrade: check-network
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m upgrade"

nix-integration-test-ledger: check-network 
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m ledger"

nix-integration-test-slow: check-network 
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m slow"

nix-integration-test-ibc: check-network 
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m ibc"

nix-integration-test-byzantine: check-network
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m byzantine"

nix-integration-test-gov: check-network 
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m gov"

nix-integration-test-grpc: check-network make-proto
	nix-shell ./integration_tests/shell.nix --run "pytest -v -m grpc"

nix-integration-test-all: check-network make-proto
	nix-shell ./integration_tests/shell.nix --run "pytest -v"


nix-build-%: check-network check-os
	@if [ -e ~/.nix/remote-build-env ]; then \
		. ~/.nix/remote-build-env; \
	fi && \
	nix-build -o $* -A $* docker.nix;
	docker load < $*;

chaindImage: nix-build-chaindImage
pystarportImage: nix-build-pystarportImage

check-network:
ifeq ($(NETWORK),mainnet)	
else ifeq ($(NETWORK),testnet)
else
	@echo "Unrecognized network: ${NETWORK}"
endif
	@echo "building network: ${NETWORK}"

check-os:
ifeq ($(OS), Darwin)
ifneq ("$(wildcard ~/.nix/remote-build-env))","")
	@echo "installed nix-remote-builder before" && \
	docker run --restart always --name nix-docker -d -p 3022:22 lnl7/nix:ssh 2> /dev/null || echo "nix-docker is already running"
else
	@echo install nix-remote-builder
	git clone https://github.com/holidaycheck/nix-remote-builder.git
	cd nix-remote-builder && ./init.sh
	rm -rf nix-remote-builder
endif
endif

###############################################################################
###                              Release                                    ###
###############################################################################
.PHONY: release-dry-run
release-dry-run:
	./scripts/release.sh

###############################################################################
###                              Documentation                              ###
###############################################################################

# generate api swagger document
document:
	make all -f MakefileDoc

# generate protobuf files
# ./proto -> ./x
proto-all:
	make proto-all -f MakefileDoc
