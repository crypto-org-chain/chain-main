local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';

{
  chaintest: {
    validators: [{ coins: value, staked: value } for value in ['10cro', '10cro']],
    accounts: default.reserves,
    hw_account: {
      name: 'hw',
      coins: '100cro',
    },
    genesis: {
      app_state: {
        staking: genesis.app_state.staking,
      },
    },
  },
}
