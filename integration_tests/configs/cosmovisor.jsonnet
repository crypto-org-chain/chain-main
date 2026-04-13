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
        tieredrewards: {
          params: {
            target_base_rewards_rate: '0.030000000000000000',
          },
          tiers: [{
            id: 1,
            exit_duration: '5s',
            bonus_apy: '0.040000000000000000',
            min_lock_amount: '1000000',
            close_only: false,
          }],
          next_position_id: '1',
        },
      },
    },
  },
}
