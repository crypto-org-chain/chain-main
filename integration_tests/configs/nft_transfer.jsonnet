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
  ],
  genesis: {},
};
local validator = {
  coins: '10cro',
  staked: '10cro',
};

{
  'ibc-0': default {
    validators: [validator { base_port: 26650 }, validator],
  },
  'ibc-1': default {
    validators: [validator { base_port: port } for port in [26750, 26760]],
  },
  'ibc-2': default {
    validators: [validator { base_port: port } for port in [26850, 26860]],
  },
  relayer: {},
}
