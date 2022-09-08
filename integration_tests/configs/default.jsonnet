local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = {
  coins: '10cro',
  staked: '10cro',
};

{
  chaintest: {
    validators: [validator { commission_rate: '0.000000000000000000' }, validator],
    accounts: default.accounts + default.reserves + default.signers + [
      {
        name: 'msigner1',
        coins: '2000cro',
      },
      {
        name: 'msigner2',
        coins: '2000cro',
      },
    ],
    genesis+: genesis,
  },
}
