local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

{
  'tieredrewards-test': {
    validators: [validator {
      commission_rate: '0.000000000000000000',
    }, validator],
    accounts: default.accounts + default.signers,
    genesis+: genesis {
      app_state+: {
        tieredrewards: {
          params: {
            target_base_rewards_rate: '0.000000000000000000',
            tiers: [{
              tier_id: 1,
              exit_commitment_duration: '15s',
              exit_commitment_duration_in_years: 1,
              bonus_apy: '0.100000000000000000',
              min_lock_amount: '1000',
            }],
          },
        },
      },
    },
  },
}
