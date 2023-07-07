local default = import 'accounts.jsonnet';
local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

{
  chaintest: {
    validators: [validator {
      commission_rate: '0.000000000000000000',
      'app-config': {
        memiavl: {
          enable: true,
          'zero-copy': true,
          'snapshot-interval': 5,
        },
        store: {
          streamers: ['versiondb'],
        },
      },
    }, validator],
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
