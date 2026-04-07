local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

{
  // Like base_rewards.jsonnet but with mint inflation enabled so the fee
  // collector is funded each block.  Used by the "fee collector sufficient"
  // integration test.
  'base-rewards-inflation-test': {
    validators: [validator {
      commission_rate: '0.000000000000000000',
    }, validator],
    accounts: default.accounts + default.signers,
    genesis+: genesis {
      app_state+: {
        // 13% inflation pinned so the mint module fills the fee collector
        // with far more than the per-block target each block.
        mint: {
          minter: {
            inflation: '0.130000000000000000',
            annual_provisions: '0.000000000000000000',
          },
          params: {
            mint_denom: 'basecro',
            inflation_rate_change: '0.000000000000000000',
            inflation_max: '0.130000000000000000',
            inflation_min: '0.130000000000000000',
            goal_bonded: '0.670000000000000000',
            blocks_per_year: '63115',
          },
        },
        tieredrewards: {
          params: {
            // 3% target -- per-block target = 2000000000 * 0.03/63115 ~ 950 basecro,
            // well below the ~2.6M basecro minted each block by the mint module.
            target_base_rewards_rate: '0.030000000000000000',
          },
        },
      },
    },
  },
}
