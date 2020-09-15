module github.com/crypto-com/chain-main

go 1.14

require (
	github.com/cosmos/cosmos-sdk v0.34.4-0.20200914065108-201787bba330
	github.com/gogo/protobuf v1.3.1
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.14.8
	github.com/imdario/mergo v0.3.11
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.6.1
	github.com/tendermint/tendermint v0.34.0-rc3.0.20200907055413-3359e0bf2f84
	github.com/tendermint/tm-db v0.6.2

)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4
