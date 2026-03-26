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
        // Increase voting_period so approve_proposal (deposit + 2 votes) completes
        // within the window. genesis.jsonnet defaults to 10s which is too tight.
        gov+: {
          params+: {
            voting_period: '30s',
            max_deposit_period: '30s',
            min_deposit: [{ denom: 'basecro', amount: '10000000' }],
          },
        },
        // Disable mint inflation for clean reward accounting.
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
            target_base_rewards_rate: '0.030000000000000000',
          },
          // Two tiers with short exit durations suitable for integration tests.
          // Tier 1: 5s exit — used for full exit-flow tests.
          // Tier 2: 30s exit — used for testing "exit not yet elapsed" rejection.
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
              exit_duration: '30s',
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
