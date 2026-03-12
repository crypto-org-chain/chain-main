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
        tieredrewards: {
          params: {
            target_base_rewards_rate: '100.000000000000000000',
          },
        },
      },
    },
  },
}
