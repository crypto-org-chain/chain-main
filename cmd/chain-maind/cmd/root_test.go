package cmd_test

import (
	"testing"

	"github.com/crypto-com/chain-main/app"
	"github.com/crypto-com/chain-main/cmd/chain-maind/cmd"
	"github.com/stretchr/testify/assert"
)

func TestConvertDenom(t *testing.T) {
	assert := assert.New(t)

	// FIXME: Should run only once on test runner boots up
	app.SetConfig()

	assert.Equal(
		cmd.ConvertDenom([]string{}),
		[]string{},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"query", "validators"}),
		[]string{"query", "validators"},
	)
}

func TestConvertDenomTx(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(
		cmd.ConvertDenom([]string{"tx", "bank", "send", "from", "to", "1000cro"}),
		[]string{"tx", "bank", "send", "from", "to", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"--home", "./.chainmaind", "tx", "bank", "send", "from", "to", "1000cro"}),
		[]string{"--home", "./.chainmaind", "tx", "bank", "send", "from", "to", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"tx", "staking", "delegate", "node-pub-key", "1000cro"}),
		[]string{"tx", "staking", "delegate", "node-pub-key", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"--home", "./.chainmaind", "tx", "staking", "delegate", "node-pub-key", "1000cro"}),
		[]string{"--home", "./.chainmaind", "tx", "staking", "delegate", "node-pub-key", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"tx", "staking", "unbond", "node-pub-key", "1000cro"}),
		[]string{"tx", "staking", "unbond", "node-pub-key", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"--home", "./.chainmaind", "tx", "staking", "unbond", "node-pub-key", "1000cro"}),
		[]string{"--home", "./.chainmaind", "tx", "staking", "unbond", "node-pub-key", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"tx", "staking", "redelegate", "from-node-pub-key", "to-node-pub-key", "1000cro"}),
		[]string{"tx", "staking", "redelegate", "from-node-pub-key", "to-node-pub-key", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{
			"--home", "./.chainmaind",
			"tx", "staking", "redelegate", "from-node-pub-key", "to-node-pub-key", "1000cro",
		}),
		[]string{
			"--home", "./.chainmaind",
			"tx", "staking", "redelegate", "from-node-pub-key", "to-node-pub-key", "100000000000basecro",
		},
	)
}

func TestConvertDenomGenTx(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(
		cmd.ConvertDenom([]string{"gentx", "--amount", "1000cro"}),
		[]string{"gentx", "--amount", "100000000000basecro"},
	)
	assert.Equal(
		cmd.ConvertDenom([]string{"--home", "./.chainmaind", "gentx", "--amount", "1000cro"}),
		[]string{"--home", "./.chainmaind", "gentx", "--amount", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"gentx", "--amount=1000cro"}),
		[]string{"gentx", "--amount=100000000000basecro"},
	)
	assert.Equal(
		cmd.ConvertDenom([]string{"--home", "./.chainmaind", "gentx", "--amount=1000cro"}),
		[]string{"--home", "./.chainmaind", "gentx", "--amount=100000000000basecro"},
	)
}

func TestConvertDenomAddGenesisAccount(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(
		cmd.ConvertDenom([]string{"add-genesis-account"}),
		[]string{"add-genesis-account"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{"add-genesis-account", "validator", "1000cro"}),
		[]string{"add-genesis-account", "validator", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{
			"add-genesis-account", "--home=\"./.chainmaind\"", "validator", "1000cro",
		}),
		[]string{"add-genesis-account", "--home=\"./.chainmaind\"", "validator", "100000000000basecro"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{
			"add-genesis-account", "--home", "\"./.chainmaind\"", "validator", "1000cro",
		}),
		[]string{"add-genesis-account", "--home", "\"./.chainmaind\"", "validator", "100000000000basecro"},
	)
}

func TestConvertDenomTestnet(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(
		cmd.ConvertDenom([]string{"testnet"}),
		[]string{"testnet"},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{
			"testnet",
			"--amount", "1000cro",
			"--staking-amount", "1000cro",
			"--vesting-amount", "1000cro",
			"--minimum-gas-prices", "1000cro",
		}),
		[]string{
			"testnet",
			"--amount", "100000000000basecro",
			"--staking-amount", "100000000000basecro",
			"--vesting-amount", "100000000000basecro",
			"--minimum-gas-prices", "100000000000basecro",
		},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{
			"--home", "\"./.chainmaind\"",
			"testnet",
			"--amount", "1000cro",
			"--staking-amount", "1000cro",
			"--vesting-amount", "1000cro",
			"--minimum-gas-prices", "1000cro",
		}),
		[]string{
			"--home", "\"./.chainmaind\"",
			"testnet",
			"--amount", "100000000000basecro",
			"--staking-amount", "100000000000basecro",
			"--vesting-amount", "100000000000basecro",
			"--minimum-gas-prices", "100000000000basecro",
		},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{
			"testnet",
			"--amount=1000cro",
			"--staking-amount=1000cro",
			"--vesting-amount=1000cro",
			"--minimum-gas-prices=1000cro",
		}),
		[]string{
			"testnet",
			"--amount=100000000000basecro",
			"--staking-amount=100000000000basecro",
			"--vesting-amount=100000000000basecro",
			"--minimum-gas-prices=100000000000basecro",
		},
	)

	assert.Equal(
		cmd.ConvertDenom([]string{
			"--home", "\"./.chainmaind\"",
			"testnet",
			"--amount=1000cro",
			"--staking-amount=1000cro",
			"--vesting-amount=1000cro",
			"--minimum-gas-prices=1000cro",
		}),
		[]string{
			"--home", "\"./.chainmaind\"",
			"testnet",
			"--amount=100000000000basecro",
			"--staking-amount=100000000000basecro",
			"--vesting-amount=100000000000basecro",
			"--minimum-gas-prices=100000000000basecro",
		},
	)
}

func TestFindCommand(t *testing.T) {
	assert := assert.New(t)

	var result string
	var found bool

	_, found = cmd.FindCommand([]string{})
	assert.False(found)

	result, found = cmd.FindCommand([]string{"add-genesis-account"})
	assert.True(found)
	assert.Equal(result, "add-genesis-account")

	_, found = cmd.FindCommand([]string{
		"--home=\"./.chainmaind\"",
	})
	assert.False(found)

	result, found = cmd.FindCommand([]string{
		"--home=\"./.chainmaind\"", "add-genesis-account",
	})
	assert.True(found)
	assert.Equal(result, "add-genesis-account")

	result, found = cmd.FindCommand([]string{
		"--home", "\"./.chainmaind\"", "add-genesis-account",
	})
	assert.True(found)
	assert.Equal(result, "add-genesis-account")

	result, found = cmd.FindCommand([]string{
		"--home=\"./.chainmaind\"", "--home", "\"./.chainmaind\"", "add-genesis-account",
	})
	assert.True(found)
	assert.Equal(result, "add-genesis-account")
}

func TestFindModuleCommand(t *testing.T) {
	assert := assert.New(t)

	var result string
	var found bool

	_, found = cmd.FindModuleCommand([]string{})
	assert.False(found)

	_, found = cmd.FindModuleCommand([]string{"tx"})
	assert.False(found)

	_, found = cmd.FindModuleCommand([]string{
		"tx", "--home=\"./.chainmaind\"",
	})
	assert.False(found)

	_, found = cmd.FindModuleCommand([]string{
		"tx", "--home=\"./.chainmaind\"", "bank",
	})
	assert.False(found)

	result, found = cmd.FindModuleCommand([]string{
		"tx", "--home=\"./.chainmaind\"", "bank", "--keyring-backend=test", "send",
	})
	assert.True(found)
	assert.Equal(result, "bank send")

	result, found = cmd.FindModuleCommand([]string{
		"tx", "--home=\"./.chainmaind\"", "bank", "send",
	})
	assert.True(found)
	assert.Equal(result, "bank send")

	result, found = cmd.FindModuleCommand([]string{
		"tx", "bank", "send",
	})
	assert.True(found)
	assert.Equal(result, "bank send")
}

func TestFindModuleCommandArgIndex(t *testing.T) {
	assert := assert.New(t)

	var result int
	var found bool

	_, found = cmd.FindModuleCommandArgIndex([]string{}, 0)
	assert.False(found)

	_, found = cmd.FindModuleCommandArgIndex([]string{}, 2)
	assert.False(found)

	_, found = cmd.FindModuleCommandArgIndex([]string{"tx"}, 1)
	assert.False(found)

	_, found = cmd.FindModuleCommandArgIndex([]string{
		"tx", "--home=\"./.chainmaind\"",
	}, 1)
	assert.False(found)

	_, found = cmd.FindModuleCommandArgIndex([]string{
		"tx", "--home=\"./.chainmaind\"", "bank",
	}, 2)
	assert.False(found)

	result, found = cmd.FindModuleCommandArgIndex([]string{
		"tx", "--home=\"./.chainmaind\"", "bank", "--keyring-backend=test", "send", "1000cro",
	}, 3)
	assert.True(found)
	assert.Equal(result, 5)

	result, found = cmd.FindModuleCommandArgIndex([]string{
		"tx", "--home=\"./.chainmaind\"", "bank", "send", "1000cro",
	}, 3)
	assert.True(found)
	assert.Equal(result, 4)

	result, found = cmd.FindModuleCommandArgIndex([]string{
		"tx", "bank", "send", "1000cro",
	}, 3)
	assert.True(found)
	assert.Equal(result, 3)
}

func TestConvertFlagAmountValueDenom(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(
		cmd.ConvertFlagAmountValueDenom([]string{"bank", "send"}, 0),
		[]string{"bank", "send"},
	)

	assert.Equal(
		cmd.ConvertFlagAmountValueDenom([]string{"bank", "send", "--amount"}, 2),
		[]string{"bank", "send", "--amount"},
	)

	assert.Equal(
		cmd.ConvertFlagAmountValueDenom([]string{"bank", "send", "--amount", "1000cro"}, 1),
		[]string{"bank", "send", "--amount", "1000cro"},
	)

	assert.Panics(func() {
		cmd.ConvertFlagAmountValueDenom([]string{"bank", "send", "--amount", "invalid"}, 2)
	})

	assert.Equal(
		cmd.ConvertFlagAmountValueDenom([]string{"bank", "send", "--amount", "1000cro"}, 2),
		[]string{"bank", "send", "--amount", "100000000000basecro"},
	)

	assert.Panics(func() {
		cmd.ConvertFlagAmountValueDenom([]string{"bank", "send", "--amount=1000cro=1000cro"}, 2)
	})

	assert.Equal(
		cmd.ConvertFlagAmountValueDenom([]string{"bank", "send", "--amount=1000cro"}, 2),
		[]string{"bank", "send", "--amount=100000000000basecro"},
	)
}
