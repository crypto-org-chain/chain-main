local config = import 'default.jsonnet';

config {
  chaintest+: {
    validators: [super.validators[0] {
      'app-config':: super['app-config'],
    }] + super.validators[1:],
    genesis+: {
      gov: {
        params: {
          voting_period: '10s',
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
}
