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
