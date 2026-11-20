local config = import 'default.jsonnet';

config {
  chaintest+: {
    validators: [validator {
      'app-config':: super['app-config'],
      client_config: {
        'broadcast-mode': 'sync',
      },
    } for validator in super.validators],
    genesis+: {
      app_state+: {
        gov: {
          voting_params: {
            // Voting must outlast approve_proposal's pre-vote CLI calls (~6s,
            // more under load); see UPGRADE_HEIGHT_MARGIN in test_upgrade.py.
            voting_period: '20s',
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
