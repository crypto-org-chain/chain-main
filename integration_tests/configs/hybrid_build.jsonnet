local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

{
  hybrid_chain: {
    validators: [
      validator {
        coins: '10cro',
        staked: '10cro',
      },
      validator {
        coins: '10cro',
        staked: '10cro',
      },
      validator {
        coins: '10cro',
        staked: '10cro',
      },
    ],
    accounts: [
      {
        name: 'community',
        coins: '100cro',
      },
      {
        name: 'signer1',
        coins: '100cro',
      },
      {
        name: 'signer2',
        coins: '100cro',
      },
    ],
    genesis+: genesis,
  },
}
