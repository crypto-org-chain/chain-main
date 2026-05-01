local genesis = import 'genesis.jsonnet';

// Chain-id contains "testnet" so the v7.1.0-testnet upgrade handler's
// "testnet" branch matches.
//
// Both binaries in upgrade-test-v7-testnet.nix are compiled with
// `-tags testnet`, so the bech32 account prefix is `tcro` and the base
// denom is `basetcro` (human unit `tcro`). Everything below uses the
// testnet-tag denoms explicitly — we cannot reuse accounts.jsonnet /
// validator.jsonnet because those hardcode `cro`/`basecro`.
{
  'upgrade-testnet-v7': {
    validators: [
      {
        coins: '40tcro',
        staked: '40tcro',
        commission_rate: '0.000000000000000000',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '10tcro',
        staked: '10tcro',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '10tcro',
        staked: '10tcro',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
    ],
    accounts: [
      {
        name: 'community',
        coins: '100tcro',
        mnemonic: 'shine blade problem hint you section hazard number skill congress harsh actress wasp hero under hair stand affair work cherry amused silver segment donor',
      },
      {
        name: 'ecosystem',
        coins: '200tcro',
        mnemonic: 'obvious more expand sell ozone dream emerge charge decade unable trigger able beyond ghost humble figure absorb other rebel shy corn manage club flock',
      },
      {
        name: 'launch',
        coins: '100tcro',
        mnemonic: 'bulb spend fence property level bicycle access prison album oppose recycle plug payment hold fly deny rhythm creek isolate panic level system cushion priority',
      },
      {
        name: 'signer1',
        coins: '10000tcro',
        mnemonic: 'story round shaft idle episode maple crash wave relax below ceiling trim boat comic collect poet squirrel robot observe gravity arm horse ankle run',
      },
      {
        name: 'signer2',
        coins: '2000tcro',
        mnemonic: 'corn foot nerve joy simple genius equal exercise follow moon radar lazy flavor name lecture panda disagree leaf course quit capital resist ostrich goat',
      },
    ],
    config: {
      consensus: {
        timeout_commit: '1s',
      },
    },
    genesis+: genesis {
      app_state+: {
        gov+: {
          params+: {
            voting_period: '10s',
            max_deposit_period: '1s',
            min_deposit: [{ denom: 'basetcro', amount: '10000000' }],
          },
        },
        mint: {
          minter: {
            inflation: '0.000000000000000000',
            annual_provisions: '0.000000000000000000',
          },
          params: {
            mint_denom: 'basetcro',
            inflation_rate_change: '0.130000000000000000',
            inflation_max: '0.000000000000000000',
            inflation_min: '0.000000000000000000',
            goal_bonded: '0.670000000000000000',
            blocks_per_year: '63115',
          },
        },
        slashing+: {
          params+: {
            signed_blocks_window: '10',
            slash_fraction_downtime: '0.01',
            downtime_jail_duration: '60s',
          },
        },
        // v7 tieredrewards state seeded at genesis — accepted by both
        // the pre-rewrite and post-rewrite binaries: Params/Tier proto
        // shapes are unchanged between the two, and the other fields
        // default to empty.
        tieredrewards: {
          params: {
            target_base_rewards_rate: '0.030000000000000000',
          },
          tiers: [
            {
              id: 1,
              exit_duration: '5s',
              bonus_apy: '0.040000000000000000',
              min_lock_amount: '1000000',
              close_only: false,
            },
            {
              id: 2,
              exit_duration: '60s',
              bonus_apy: '0.020000000000000000',
              min_lock_amount: '5000000',
              close_only: false,
            },
          ],
          next_position_id: '1',
        },
      },
    },
  },
}
