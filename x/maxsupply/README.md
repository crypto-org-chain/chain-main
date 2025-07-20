# MaxSupply Module

## Overview

The MaxSupply module is a custom module for the Chain-main blockchain that manages and queries the maximum supply parameters of the blockchain. This module provides a simple and effective way to set and query the maximum supply limit for tokens.

## Features

- **Maximum Supply Management**: Set and maintain the maximum supply parameters for tokens
- **Query Interface**: Provides gRPC and REST API to query maximum supply
- **Governance Integration**: Supports updating module parameters through governance proposals
- **Genesis State Management**: Supports initializing module state in the genesis block

## Module Structure

x/maxsupply/
├── client/cli/          # CLI commands
├── keeper/              # Business logic and state management
├── types/               # Type definitions and codecs
├── abci.go             # ABCI lifecycle handling
├── genesis.go          # Genesis state handling
├── module.go           # Module definition
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

#### Request Format

```(go)
    message QueryMaxSupplyRequest {}
```

#### Response Format

```(go)
message QueryMaxSupplyResponse {
  string max_supply = 1;
}
```

### REST API (gRPC Gateway)

#### Query Maximum Supply Example

```(bash)
# GET request
curl http://localhost:1317/chainmain/maxsupply/v1/params/max_supply
```

#### Response Example

```(bash)
{
  "max_supply": "1000000000000000000000000000"
}
```

### CLI Commands

#### Query Maximum Supply CLI

```(bash)
# Query current maximum supply
./build/chain-maind query maxsupply max-supply

# Using specific node
./build/chain-maind query maxsupply max-supply --node tcp://localhost:26657
```

## Governance Operations

### Update Module Parameters

The MaxSupply module supports updating parameters through governance proposals(See test_max_supply_update_via_governance)
