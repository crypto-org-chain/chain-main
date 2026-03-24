local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

{
  'base-rewards-test': {
    validators: [validator {
      commission_rate: '0.000000000000000000',
    }, validator],
    accounts: default.accounts + default.signers,
    genesis+: genesis {
      app_state+: {
        // target_base_rewards_rate must be <= 1.0; use reduced blocks_per_year to preserve per-block target.
        mint+: {
          params+: {
            blocks_per_year: '63115',
          },
        },
        tieredrewards: {
          params: {
            target_base_rewards_rate: '1.000000000000000000',
          },
        },
      },
    },
  },
}
