module github.com/crypto-com/chain-main

go 1.15

require (
	github.com/cosmos/cosmos-sdk v0.34.4-0.20200923230655-09998ef86e28
	github.com/gogo/protobuf v1.3.1
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.15.0
	github.com/imdario/mergo v0.3.11
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.7
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/tendermint/tendermint v0.34.0-rc3.0.20200923104252-a2bbc2984bcc
	github.com/tendermint/tm-db v0.6.2

)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4
