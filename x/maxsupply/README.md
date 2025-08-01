# MaxSupply Module

## Overview

The MaxSupply module is a custom module for the Chain-main blockchain that manages and queries the maximum supply parameters of the blockchain. This module provides a simple and effective way to set and query the maximum supply limit for tokens.

## Features

- **Maximum Supply Management**: Set and maintain the maximum supply parameters for tokens
- **Query Interface**: Provides gRPC and REST API to query maximum supply
- **Governance Integration**: Supports updating module parameters through governance proposals
- **Genesis State Management**: Supports initializing module state in the genesis block

## Supply Calculation

The module implements a comprehensive supply calculation system:

### Maximum Supply

- **Definition**: The absolute maximum number of tokens that can ever exist
- **Purpose**: Sets an upper bound for the chain base token supply operations
- **Format**: Stored as `cosmossdk.io/math.Int` for precise large number handling

### Burned Addresses

- **Definition**: A list of addresses that hold tokens considered "burned" or permanently removed from circulation
- **Purpose**: Exclude burned tokens from circulating supply calculations
- **Format**: Array of bech32-encoded addresses (e.g., `["cro1abc...", "cro1def..."]`)

### Circulating Supply Calculation

Circulating Supply = Total Supply - Sum(Balances of Burned Addresses)

## Module Structure

x/maxsupply/\
├── client/cli/          # CLI commands\
├── keeper/              # Business logic and state management\
├── types/               # Type definitions and codecs\
├── abci.go             # ABCI lifecycle handling\
├── genesis.go          # Genesis state handling\
├── module.go           # Module definition\
└── README.md           # This file

## API Interface

### gRPC Queries

The MaxSupply module provides the following gRPC query interface:

#### Query Maximum Supply

```(go)
service Query {
  rpc MaxSupply(QueryMaxSupplyRequest) returns (QueryMaxSupplyResponse) {
    option (google.api.http).get = "/chainmain/maxsupply/v1/params/max_supply";
  }
}
```

```(go)
    message QueryMaxSupplyRequest {}
```

```(go)
message QueryMaxSupplyResponse {
  string max_supply = 1;
}
```

#### Query Burned Addresses

```(go)
  rpc BurnedAddresses(QueryBurnedAddressesRequest) returns (QueryBurnedAddressesResponse) {
    option (google.api.http).get = "/chainmain/maxsupply/v1/params/burned_addresses";
  }
```

```(go)
    message QueryBurnedAddressesRequest {}
```

```(go)
message QueryBurnedAddressesResponse {
  repeated string burned_addresses = 1;
}
```

### REST API (gRPC Gateway)

#### Query Example

```(bash)
# GET request
curl http://localhost:1317/chainmain/maxsupply/v1/params/max_supply

curl http://localhost:1317/chainmain/maxsupply/v1/params/burned_addresses
```

#### Response Example

```(bash)
{
  "max_supply": "1000000000000000000000000000"
}

{
  "burned_addresses": [
    "cro1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqtcgxmv"
  ]
}
```

### CLI Commands

#### Query Maximum Supply CLI

```(bash)
# Query current maximum supply
./build/chain-maind query maxsupply max-supply

# Query burned addresses list
./build/chain-maind query maxsupply burned-addresses


# Using specific node
./build/chain-maind query maxsupply max-supply --node tcp://localhost:26657
```

## Governance Operations

### Update Module Parameters

The MaxSupply module supports updating parameters through governance proposals(See test_max_supply_update_via_governance)
