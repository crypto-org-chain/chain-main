local genesis = import 'genesis.jsonnet';
local validator = import 'validator.jsonnet';

{
  'inflation-test': {
    validators: [validator {
      coins: '10cro',
      staked: '10cro',
    }],
    accounts: [
      {
        // supply holder — bulk of the ~98.7B CRO
        name: 'supply',
        coins: '98700000000cro',
        mnemonic: 'upgrade fabric citizen asthma card oxygen blue board spatial appear cousin bench next win alone',
      },
      {
        // burned address: cro17w87d4yyyjys26v9a4r83a9e5eeamjg5hp5z6g
        name: 'burned',
        coins: '380000000cro',
        mnemonic: 'pelican proof school bus zoo garbage height various miss please arm warfare story damage sort',
      },
    ],
    config: {
      consensus: {
        timeout_commit: '5ms',
      },
    },
    genesis+: genesis {
      app_state+: {
        mint: {
          minter: {
            inflation: '0.010000000000000000',
          },
          params: {
            mint_denom: 'basecro',
            inflation_rate_change: '0.100000000000000000',
            inflation_max: '0.010000000000000000',
            inflation_min: '0.010000000000000000',
            goal_bonded: '0.600000000000000000',
          },
        },
        inflation: {
          params: {
            // 100B CRO * 10^8 basecro
            max_supply: '10000000000000000000',
            // burned address derived from the 'burned' account mnemonic above
            burned_addresses: ['cro17w87d4yyyjys26v9a4r83a9e5eeamjg5hp5z6g'],
            decay_rate: '0.068000000000000000',
          },
          decay_epoch_start: '1',
        },
      },
    },
  },
}
