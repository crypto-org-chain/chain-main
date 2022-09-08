local ibc = import 'ibc.jsonnet';
local sequence_config = {
  channel_id: 'channel-0',
  port_id: 'transfer',
  sequence: '1',
};
local modules = ['ibc', 'transfer'];
local genesis = {
  app_state+: {
    capability: {
      index: '3',
      owners: [
        {
          index: '1',
          index_owners: {
            owners: [{ module: module, name: 'ports/transfer' } for module in modules],
          },
        },
        {
          index: '2',
          index_owners: {
            owners: [{ module: module, name: 'capabilities/ports/transfer/channels/channel-0' } for module in modules],
          },
        },
      ],
    },
    ibc: {
      channel_genesis: {
        ack_sequences: [sequence_config],
        acknowledgements: [],
        channels: [
          {
            channel_id: 'channel-0',
            connection_hops: ['connection-0'],
            counterparty: {
              channel_id: 'channel-0',
              port_id: 'transfer',
            },
            ordering: 'ORDER_UNORDERED',
            port_id: 'transfer',
            state: 'STATE_OPEN',
            version: 'ics20-1',
          },
        ],
        commitments: [],
        next_channel_sequence: '1',
        receipts: [],
        recv_sequences: [sequence_config],
        send_sequences: [sequence_config],
      },
      connection_genesis: {
        client_connection_paths: [
          {
            client_id: '07-tendermint-0',
            paths: ['connection-0'],
          },
        ],
        connections: [
          {
            client_id: '07-tendermint-0',
            counterparty: {
              client_id: '07-tendermint-0',
              connection_id: 'connection-0',
              prefix: {
                key_prefix: 'aWJj',
              },
            },
            delay_period: '0',
            id: 'connection-0',
            state: 'STATE_OPEN',
            versions: [
              {
                features: [
                  'ORDER_ORDERED',
                  'ORDER_UNORDERED',
                ],
                identifier: '1',
              },
            ],
          },
        ],
        next_connection_sequence: '1',
      },
    },
  },
};

ibc {
  'ibc-0'+: {
    genesis+: genesis,
  },
  'ibc-1'+: {
    accounts: [
      {
        name: 'relayer',
        coins: '100cro,100ibc/6411AE2ADA1E73DB59DB151A8988F9B7D5E7E233D8414DB6817F8F1A01611F86',
      },
      {
        name: 'signer',
        coins: '200cro',
      },
    ],
    genesis+: genesis,
  },
}
