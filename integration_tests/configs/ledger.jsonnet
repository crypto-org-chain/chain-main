local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

{
  chaintest: {
    validators: [validator, validator],
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
