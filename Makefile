PACKAGES=$(shell go list ./... | grep -v '/simulation')
PACKAGE_NAME:=github.com/crypto-org-chain/chain-main
GOLANG_CROSS_VERSION  = v1.16.4


VERSION := $(shell echo $(shell git describe --tags 2>/dev/null ) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
NETWORK ?= mainnet
COVERAGE ?= coverage.txt
BUILDDIR ?= $(CURDIR)/build
LEDGER_ENABLED ?= true

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=crypto-org-chain-chain \
	-X github.com/cosmos/cosmos-sdk/version.ServerName=chain-maind \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)

BUILD_FLAGS := -ldflags '$(ldflags)'
TESTNET_FLAGS ?=

ledger ?= HID
ifeq ($(LEDGER_ENABLED),true)
	BUILD_TAGS := -tags cgo,ledger,!test_ledger_mock,!ledger_mock
	ifeq ($(ledger), ZEMU)
		BUILD_TAGS := $(BUILD_TAGS),ledger_zemu
	else
		BUILD_TAGS := $(BUILD_TAGS),!ledger_zemu
	endif
endif

ifeq ($(NETWORK),testnet)
	BUILD_TAGS := $(BUILD_TAGS),testnet
	TEST_TAGS := "--tags=testnet"
endif

SIMAPP = github.com/crypto-org-chain/chain-main/v2/app
BINDIR ?= ~/go/bin

OS := $(shell uname)

all: install

install: check-network go.sum
		go install -mod=readonly $(BUILD_FLAGS) $(BUILD_TAGS) ./cmd/chain-maind

build: check-network go.sum
		go build -mod=readonly $(BUILD_FLAGS) $(BUILD_TAGS) -o $(BUILDDIR)/chain-maind ./cmd/chain-maind

buildwindows: check-network go.sum
		go build -buildmode=exe -mod=readonly $(BUILD_FLAGS) $(BUILD_TAGS) -o $(BUILDDIR)/chain-maind ./cmd/chain-maind


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
	@go test $(TEST_TAGS) -v -mod=readonly $(PACKAGES) -coverprofile=$(COVERAGE) -covermode=atomic
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
	@nix run -f ./. lint-env -c lint-ci

test-sim-nondeterminism: check-network
	@echo "Running non-determinism test..."
	@go test $(TEST_TAGS) -mod=readonly $(SIMAPP) -run TestAppStateDeterminism -Enabled=true \
		-NumBlocks=100 -BlockSize=200 -Commit=true -Period=0 -v -timeout 24h

test-sim-custom-genesis-fast: check-network
	@echo "Running custom genesis simulation..."
	@echo "By default, ${HOME}/.chain-maind/config/genesis.json will be used."
	@go test $(TEST_TAGS) -mod=readonly $(SIMAPP) -run TestFullAppSimulation -Genesis=${HOME}/.gaiad/config/genesis.json \
		-Enabled=true -NumBlocks=100 -BlockSize=200 -Commit=true -Seed=99 -Period=5 -v -timeout 24h

test-sim-import-export:
	@echo "Running Chain import/export simulation. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) 25 5 TestAppImportExport

test-sim-after-import:
	@echo "Running application simulation-after-import. This may take several minutes..."
	@$(BINDIR)/runsim -Jobs=4 -SimAppPkg=$(SIMAPP) 50 5 TestAppSimulationAfterImport

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
	nix run -f ./. run-integration-tests -c run-integration-tests

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
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-v ${GOPATH}/pkg:/go/pkg \
		-w /go/src/$(PACKAGE_NAME) \
		troian/golang-cross:${GOLANG_CROSS_VERSION} \
		--rm-dist --skip-validate --skip-publish

.PHONY: release
release:
	@if [ ! -f ".release-env" ]; then \
		echo "\033[91m.release-env is required for release\033[0m";\
		exit 1;\
	fi
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		--env-file .release-env \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		troian/golang-cross:${GOLANG_CROSS_VERSION} \
		release --rm-dist --skip-validate

###############################################################################
###                              Documentation                              ###
###############################################################################

include third_party/cosmos-sdk/contrib/devtools/Makefile

containerProtoVer=v0.2
containerProtoImage=tendermintdev/sdk-proto-gen:$(containerProtoVer)

proto-swagger-gen:
	@echo "Generating Protobuf Swagger"
	@docker run --rm -v $(CURDIR):/workspace --workdir /workspace $(containerProtoImage) sh ./scripts/protoc-swagger-gen.sh
	@echo "Patch swagger documentations"
	@sed -i"" "s/cosmosvaloper16xyempempp92x9hyzz9wrgf94r6j9h5f2w4n2l/crocncl16xyempempp92x9hyzz9wrgf94r6j9h5f2w4n2l/" app/docs/swagger-ui/swagger.yaml
	@sed -i"" "s/cosmos16xyempempp92x9hyzz9wrgf94r6j9h5f06pxxv/cro16xyempempp92x9hyzz9wrgf94r6j9h5f06pxxv/" app/docs/swagger-ui/swagger.yaml
	@sed -i"" "s/cosmosvalconspub1zcjduepq0vu2zgkgk49efa0nqwzndanq5m4c7pa3u4apz4g2r9gspqg6g9cs3k9cuf/crocnclconspub1zcjduepq0vu2zgkgk49efa0nqwzndanq5m4c7pa3u4apz4g2r9gspqg6g9cs3k9cuf/" app/docs/swagger-ui/swagger.yaml
	@sed -i"" "s/example: Cosmos-Hub/example: crypto-org-chain-mainnet/" app/docs/swagger-ui/swagger.yaml

proto-gen:
	@echo "Generating Protobuf files"
	@docker run --rm -v $(CURDIR):/workspace --workdir /workspace $(containerProtoImage) sh ./scripts/protocgen

proto-format:
	@echo "Formatting Protobuf files"
	find ./ -not -path "./third_party/*" -name *.proto -exec clang-format -i {} \;

proto-lint:
	@docker run --rm -v $(CURDIR):/workspace --workdir /workspace bufbuild/buf lint --error-format=json

proto-all: proto-format proto-lint proto-gen

update-swagger-docs: statik
	$(BINDIR)/statik -src=app/docs/swagger-ui -dest=app/docs -f -m
	@if [ -n "$(git status --porcelain)" ]; then \
        echo "\033[91mSwagger docs are out of sync!!!\033[0m";\
        exit 1;\
    else \
        echo "\033[92mSwagger docs are in sync\033[0m";\
    fi

.PHONY: proto-all proto-gen proto-swagger-gen proto-format proto-lint update-swagger-docs
