local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';
{
  stakingtest: {
    validators: [validator, validator] + [{
      coins: '1cro',
      staked: '1cro',
      min_self_delegation: 10000000,  // 0.1cro
      client_config: {
        'broadcast-mode': 'sync',
      },
    }],
    accounts: default.accounts + [
      {
        name: 'reserve',
        coins: '200cro',
        vesting: '3600s',
      },
    ] + default.signers,
    genesis+: genesis,
  },
}
