PACKAGES=$(shell go list ./... | grep -v '/simulation')

VERSION := $(shell echo $(shell git describe --tags 2>/dev/null ) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
COVERAGE?=
OUTPUT?=build/

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=crypto-com-chain \
	-X github.com/cosmos/cosmos-sdk/version.ServerName=chain-maind \
	-X github.com/cosmos/cosmos-sdk/version.ClientName=chain-maincli \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) 

BUILD_FLAGS := -ldflags '$(ldflags)'

SIMAPP = github.com/crypto-com/chain-main/app
BINDIR ?= ~/go/bin

all: install

install: go.sum
		go install -mod=readonly $(BUILD_FLAGS) ./cmd/chain-maind
		go install -mod=readonly $(BUILD_FLAGS) ./cmd/chain-maincli

build: go.sum
		go build -mod=readonly $(BUILD_FLAGS) -o $(OUTPUT)/chain-maind ./cmd/chain-maind 
		go build -mod=readonly $(BUILD_FLAGS) -o $(OUTPUT)/chain-maincli ./cmd/chain-maincli 
.PHONY: build

go.sum: go.mod
		@echo "--> Ensure dependencies have not been modified"
		GO111MODULE=on go mod verify

test:
	@go test -mod=readonly $(PACKAGES) -coverprofile=$(COVERAGE) -covermode=atomic

# look into .golangci.yml for enabling / disabling linters
lint:
	@echo "--> Running linter"
	@golangci-lint run
	@go mod verify

test-sim-nondeterminism:
	@echo "Running non-determinism test..."
	@go test -mod=readonly $(SIMAPP) -run TestAppStateDeterminism -Enabled=true \
		-NumBlocks=100 -BlockSize=200 -Commit=true -Period=0 -v -timeout 24h

test-sim-custom-genesis-fast:
	@echo "Running custom genesis simulation..."
	@echo "By default, ${HOME}/.chain-maind/config/genesis.json will be used."
	@go test -mod=readonly $(SIMAPP) -run TestFullAppSimulation -Genesis=${HOME}/.gaiad/config/genesis.json \
		-Enabled=true -NumBlocks=100 -BlockSize=200 -Commit=true -Seed=99 -Period=5 -v -timeout 24h

test-sim-import-export:
	@echo "Running Chain import/export simulation. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) 25 5 TestAppImportExport

test-sim-after-import:
	@echo "Running application simulation-after-import. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) 50 5 TestAppSimulationAfterImport

clean:
	rm -rf $(OUTPUT)/

clean-docker-compose:
	rm -rf .validator*