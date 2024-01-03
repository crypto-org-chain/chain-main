local config = import 'default.jsonnet';

config {
  chaintest+: {
    validators: [validator {
      'app-config':: super['app-config'],
      client_config: {
        'broadcast-mode': 'block',
      },
    } for validator in super.validators],
    genesis+: {
      app_state+: {
        gov: {
          voting_params: {
            voting_period: '10s',
          },
          deposit_params: {
            max_deposit_period: '10s',
            min_deposit: [
              {
                denom: 'basecro',
                amount: '10000000',
              },
            ],
          },
        },
      },
    },
  },
}
