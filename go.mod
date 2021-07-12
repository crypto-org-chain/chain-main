module github.com/crypto-org-chain/chain-main/v2

go 1.16

require (
	github.com/confluentinc/bincover v0.1.0
	github.com/cosmos/cosmos-sdk v0.42.7
	github.com/cosmos/ledger-go v0.9.2 // indirect
	github.com/gogo/protobuf v1.3.3
	github.com/golang/protobuf v1.5.2
	github.com/google/renameio v1.0.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/imdario/mergo v0.3.11
	github.com/rakyll/statik v0.1.7
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/tendermint v0.34.11
	github.com/tendermint/tm-db v0.6.4
	github.com/tidwall/gjson v1.7.5
	google.golang.org/genproto v0.0.0-20210114201628-6edceaf6022f
	google.golang.org/grpc v1.37.0

)

// TODO: https://github.com/cosmos/cosmos-sdk/pull/8388/files#r560319528
replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

replace github.com/cosmos/ledger-cosmos-go => github.com/crypto-com/ledger-cosmos-go v0.9.10-0.20200929055312-01e1d341de0f

// TODO: fix keyring upstream
replace github.com/99designs/keyring => github.com/crypto-org-chain/keyring v1.1.6-fixes

replace github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go/v4 v4.0.0-preview1
