local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';

{
  slashingtest: {
    validators: [{ coins: value, staked: value } for value in ['40cro', '10cro', '10cro']],
    accounts: default.accounts + default.reserves,
    genesis: {
      app_state: {
        staking: genesis.app_state.staking,
        slashing: {
          params: {
            signed_blocks_window: '10',
            slash_fraction_downtime: '0.01',
            downtime_jail_duration: '60s',
          },
        },
      },
    },
  },
}
