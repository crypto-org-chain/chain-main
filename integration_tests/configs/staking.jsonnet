local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
{
  stakingtest: {
    validators: [{ coins: value, staked: value } for value in ['10cro', '10cro']] + [{
      coins: '1cro',
      staked: '1cro',
      min_self_delegation: 10000000,  // 0.1cro
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
