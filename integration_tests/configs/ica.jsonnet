local ibc = import 'ibc.jsonnet';
local genesis = {
  genesis: {
    app_state: {
      interchainaccounts: {
        host_genesis_state: {
          params: {
            allow_messages: [
              '/cosmos.bank.v1beta1.MsgSend',
            ],
          },
        },
      },
    },
  },
};

{
  'ica-controller-1': ibc['ibc-0'] + genesis,
  'ica-host-1': ibc['ibc-1'] + genesis,
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
  },
}
