local genesis = import 'genesis.jsonnet';
local default = {
  accounts: [
    {
      name: 'relayer',
      coins: '100cro',
    },
    {
      name: 'signer',
      coins: '200cro',
    },
    {
      name: 'signer2',
      coins: '2000cro,100000000000ibcfee',
    },
  ],
  genesis: genesis {
    app_state+: {
      staking: {
        params: {
          unbonding_time: '1814400s',
        },
      },
      transfer: {
        params: {
          receive_enabled: true,
          send_enabled: true,
        },
      },
    },
  },
};
local validator = import 'validator.jsonnet';

{
  'ibc-0': default {
    validators: [validator { base_port: 26650 }, validator],
  },
  'ibc-1': default {
    validators: [validator { base_port: port } for port in [26750, 26760]],
  },
  relayer: {},
}
