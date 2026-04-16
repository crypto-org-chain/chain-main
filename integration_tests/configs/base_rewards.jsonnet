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
        // Pin max=min=0 so no new coins enter the fee collector during these tests.
        mint: {
          minter: {
            inflation: '0.000000000000000000',
            annual_provisions: '0.000000000000000000',
          },
          params: {
            mint_denom: 'basecro',
            inflation_rate_change: '0.130000000000000000',
            inflation_max: '0.000000000000000000',
            inflation_min: '0.000000000000000000',
            goal_bonded: '0.670000000000000000',
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
