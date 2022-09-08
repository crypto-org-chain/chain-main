{
  'devnet-solomachine': {
    validators: [
      {
        coins: '1000cro',
        staked: '1000cro',
      },
    ],
    accounts: [
      {
        name: 'solo-signer',
        mnemonic: 'awesome there minute cash write immune tag reopen price congress trouble reunion south wisdom donate credit below leave wisdom eagle sail siege rice train',
        coins: '1500cro',
        consensus: {
          timeout_commit: '5s',
        },
      },
    ],
    genesis: {
      consensus_params: {
        block: {
          max_bytes: '1048576',
          max_gas: '81500000',
          time_iota_ms: '1000',
        },
        evidence: {
          max_age_num_blocks: '403200',
          max_age_duration: '2419200000000000',
          max_bytes: '150000',
        },
      },
      app_state: {
        distribution: {
          params: {
            community_tax: '0',
            base_proposer_reward: '0',
            bonus_proposer_reward: '0',
          },
        },
        gov: {
          deposit_params: {
            max_deposit_period: '21600000000000ns',
            min_deposit: [
              {
                denom: 'basecro',
                amount: '2000000000000',
              },
            ],
          },
          voting_params: {
            voting_period: '21600000000000ns',
          },
        },
        mint: {
          minter: {
            inflation: '0.000000000000000000',
          },
          params: {
            blocks_per_year: '6311520',
            mint_denom: 'basecro',
            inflation_rate_change: '0',
            inflation_max: '0',
            inflation_min: '0',
            goal_bonded: '1',
          },
        },
        slashing: {
          params: {
            downtime_jail_duration: '3600s',
            min_signed_per_window: '0.5',
            signed_blocks_window: '5000',
            slash_fraction_double_sign: '0',
            slash_fraction_downtime: '0',
          },
        },
        staking: {
          params: {
            bond_denom: 'basecro',
            historical_entries: '10000',
            max_entries: '7',
            max_validators: '50',
            unbonding_time: '2419200000000000ns',
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
  },
}
