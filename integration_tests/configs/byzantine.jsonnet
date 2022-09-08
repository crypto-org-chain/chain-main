local genesis = import 'genesis.jsonnet';

{
  chain_id_1: {
    validators: [{ coins: value, staked: value } for value in ['8000cro', '2000cro', '1000cro']],
    accounts: [{
      name: 'wallet-1',
      coins: '20000cro',
    }],
    genesis+: genesis,
  },
}
