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
      name: 'ecosystem',
      coins: '200cro',
    },
    {
      name: 'community',
      coins: '100cro',
    },
    {
      name: 'signer2',
      coins: '2000cro,100000000000ibcfee',
    },
  ],
  genesis: {
    app_state: {
      transfer: {
        params: {
          receive_enabled: true,
          send_enabled: true,
        },
      },
      gov: genesis.app_state.gov,
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
  relayer: {
    mode: {
      clients: {
        enabled: true,
        refresh: true,
        misbehaviour: true,
      },
      connections: {
        enabled: true,
      },
      channels: {
        enabled: true,
      },
      packets: {
        enabled: true,
        tx_confirmation: true,
      },
    },
    rest: {
      enabled: true,
      host: '127.0.0.1',
      port: 3000,
    },
  },
}
