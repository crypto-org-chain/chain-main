// CAN BE REMOVED ONCE MAINNET IS UPGRADED TO v7.2.0
local accounts = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

// Mainnet-tag config (cro/basecro denom). Chain-id contains "mainnet" so
// the v7 upgrade handler's mainnet branch runs (see app/upgrades.go).
{
  'mainnet-upgrade-v7': {
    validators: [validator {
      commission_rate: '0.000000000000000000',
      client_config: {
        'broadcast-mode': 'sync',
      },
    }, validator {
      client_config: {
        'broadcast-mode': 'sync',
      },
    }],
    accounts: accounts.accounts + accounts.signers + accounts.reserves,
    config: {
      consensus: {
        timeout_commit: '1s',
      },
    },
    genesis+: genesis,
  },
}
