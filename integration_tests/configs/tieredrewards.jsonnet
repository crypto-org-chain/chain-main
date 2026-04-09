local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

{
  'tieredrewards-test': {
    validators: [validator {
      coins: '40cro',
      staked: '40cro',
      commission_rate: '0.000000000000000000',
    }, validator, validator],
    accounts: default.accounts + default.signers + default.reserves,
    config: {
      consensus: {
        timeout_commit: '1s',
      },
    },
    genesis+: genesis {
      app_state+: {
        gov+: {
          params+: {
            voting_period: '10s',
            max_deposit_period: '1s',
            min_deposit: [{ denom: 'basecro', amount: '10000000' }],
          },
        },
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
        slashing+: {
          params+: {
            signed_blocks_window: '10',
            slash_fraction_downtime: '0.01',
            downtime_jail_duration: '60s',
          },
        },
        tieredrewards: {
          params: {
            target_base_rewards_rate: '0.030000000000000000',
          },
          tiers: [
            {
              id: 1,
              exit_duration: '5s',
              bonus_apy: '0.040000000000000000',
              min_lock_amount: '1000000',
              close_only: false,
            },
            {
              id: 2,
              exit_duration: '60s',
              bonus_apy: '0.020000000000000000',
              min_lock_amount: '5000000',
              close_only: false,
            },
          ],
          next_position_id: '1',
        },
      },
    },
  },
}
