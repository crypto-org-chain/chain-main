# Inflation Module

## Overview

The Inflation module is a custom module for the Chain-main blockchain that manages inflation parameters, maximum supply limits, and implements exponential decay for inflation rates. This module provides comprehensive control over token supply and inflation mechanics.

## Features

- **Maximum Supply Management**: Set and maintain the maximum supply parameters for tokens
- **Burned Address Tracking**: Track addresses that hold burned tokens excluded from circulation
- **Inflation Decay**: Apply continuous exponential decay to inflation rates over time
- **Query Interface**: Provides gRPC and REST API to query module parameters
- **Governance Integration**: Supports updating module parameters through governance proposals
- **Genesis State Management**: Supports initializing module state in the genesis block

## Supply Calculation

The module implements a comprehensive supply calculation system:

### Maximum Supply

- **Definition**: The absolute maximum number of tokens that can ever exist
- **Purpose**: Sets an upper bound for the chain base token supply operations
- **Format**: Stored as `cosmossdk.io/math.Int` for precise large number handling
- **Default**: `0` (unlimited supply)

### Burned Addresses

- **Definition**: A list of addresses that hold tokens considered "burned" or permanently removed from circulation
- **Purpose**: Exclude burned tokens from circulating supply calculations
- **Format**: Array of bech32-encoded addresses (e.g., `["cro1abc...", "cro1def..."]`)
- **Validation**: Addresses must be valid bech32 format, non-empty, and unique

### Circulating Supply Calculation

Circulating Supply = Total Supply - Sum(Balances of Burned Addresses)

## Inflation Decay

The module implements a continuous exponential decay mechanism for inflation rates, allowing for gradual reduction of inflation over time.

### Decay Parameters

- **Decay epoch**: The block height when decay begins is stored in module state under `DecayEpochStartKey` (not a governance parameter). The v7.0.0 upgrade handler sets it to the upgrade activation height; genesis export/import carries it as `decay_epoch_start` for new chains or state exports. Chains that enable decay from genesis JSON must set `decay_epoch_start` there as well.

- **Decay Rate**: The monthly decay rate applied to inflation
  - Range: `0` to `1` (inclusive)
  - `0` = decay disabled (no reduction, follows base inflation from mint module)
  - `1` = 100% decay (no minting at all)

### Decay Formula

The inflation rate is calculated using the following formula:

```
inflation_rate = base_rate × (1 - monthly_decay)^months_elapsed
```

Where:
- `base_rate`: The inflation rate calculated using the default Cosmos SDK method
- `monthly_decay`: The decay rate parameter (0-1)
- `blocks_elapsed`: `current_block_height` - `decay_epoch_start` (from module store)
- `months_elapsed`: Continuous decimal value calculated as `blocks_elapsed / blocks_per_month`

### Decay Behavior

- **Before the decay epoch** (or when decay is disabled): Base inflation rate is used (no decay applied)
- **From the decay epoch onward** (with positive decay rate): Exponential decay is compounded continuously (every block) based on elapsed time

## Module Structure

```
x/inflation/
├── client/cli/          # CLI commands
├── keeper/              # Business logic and state management
│   ├── keeper.go       # Core keeper functions
│   ├── mint.go         # Inflation decay calculation
│   └── grpc_query.go   # gRPC query handlers
├── types/               # Type definitions and codecs
│   ├── params.go       # Parameter definitions and validation
│   └── params.pb.go    # Generated protobuf code
├── abci.go             # ABCI lifecycle handling
├── genesis.go          # Genesis state handling
├── module.go           # Module definition
└── README.md           # This file
```

## API Interface

### gRPC Queries

The Inflation module provides the following gRPC query interface:

#### Query Parameters

```protobuf
service Query {
  rpc Params(QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/chainmain/inflation/v1/params";
  }
}
```

```protobuf
message QueryParamsRequest {}
```

```protobuf
message QueryParamsResponse {
  Params params = 1;
}
```

The `Params` message includes:
- `max_supply`: Maximum supply of tokens (string, cosmos.Int format)
- `burned_addresses`: List of burned addresses (repeated string)
- `decay_rate`: Monthly decay rate (string, cosmos.Dec format, 0-1)

Genesis state also includes `decay_epoch_start` when decay has been enabled (for export/import).

### REST API (gRPC Gateway)

#### Query Example

```bash
# GET request to query all parameters
curl http://localhost:1317/chainmain/inflation/v1/params
```

#### Response Example

```json
{
  "params": {
    "max_supply": "1000000000000000000000000000",
    "burned_addresses": [
      "cosmos1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrgrp2"
    ],
    "decay_rate": "0.065"
  }
}
```

### CLI Commands

#### Query Parameters CLI

```bash
# Query current module parameters
./build/chain-maind query inflation params

# Using specific node
./build/chain-maind query inflation params --node tcp://localhost:26657
```

## Governance Operations

### Update Module Parameters

The Inflation module supports updating parameters through governance proposals. Parameters that can be updated include:

- **Max Supply**: Maximum token supply limit
- **Burned Addresses**: List of addresses holding burned tokens
- **Decay Rate**: Monthly inflation decay rate (0-1)

All parameter updates must pass validation:
- Max supply must be non-negative
- Burned addresses must be valid bech32 format, non-empty, and unique
- Decay rate must be between 0 and 1 (inclusive)

## Integration with Mint Module

The Inflation module integrates with the Cosmos SDK's Mint module by providing a custom `InflationCalculationFn` (`DeflationCalculationFn`) that applies exponential decay to the base inflation rate. The decay calculation:

1. Retrieves the base inflation rate using the default Cosmos SDK calculation
2. Checks if decay is enabled (decay rate > 0) and if current height >= stored decay epoch
3. Calculates months elapsed since the decay epoch
4. Applies exponential decay: `base_rate × (1 - decay_rate)^months_elapsed`
5. Returns the final inflation rate

This allows for smooth, continuous reduction of inflation over time while maintaining compatibility with the standard Cosmos SDK minting mechanism.
