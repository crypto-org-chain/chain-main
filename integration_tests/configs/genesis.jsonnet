{
  app_state: {
    staking: {
      params: {
        unbonding_time: '10s',
      },
    },
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
}
