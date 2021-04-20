package types_test

import (
	"errors"
	"testing"

	"github.com/crypto-org-chain/chain-main/v2/x/subscription/types"
	"github.com/stretchr/testify/require"
)

type simpleTestCase struct {
	spec     string
	roundUps map[int64]int64
}

func TestCronSpecSimpleCases(t *testing.T) {
	cases := []simpleTestCase{
		{
			spec: "* * * * *", // At every minute
			roundUps: map[int64]int64{
				59: 60,
				60: 60,
				61: 120,
			},
		},
		{
			spec: "1,5 1,5 * * *", // At minute 1 and 5 past hour 1 and 5
			roundUps: map[int64]int64{
				3 * 60:        3600 + 60,
				2*3600 + 3*60: 5*3600 + 60,
				5*3600 + 3*60: 5*3600 + 5*60,
			},
		},
		// leap year
		{
			spec: "0 0 29 2 *", // At 00:00 on day-of-month 29
			roundUps: map[int64]int64{
				0: types.TimeStruct{
					Year: 1972, Month: 2, Mday: 29,
				}.Timestamp(),
				types.TimeStruct{
					Year: 2000, Mday: 1, Month: 3,
				}.Timestamp(): types.TimeStruct{
					Year: 2004, Mday: 29, Month: 2,
				}.Timestamp(),
			},
		},
		// edge month days
		{
			spec: "0 0 29,31 3,4 *", // At 00:00 on day-of-month 29 and 31 in March and April
			roundUps: map[int64]int64{
				types.TimeStruct{
					Year: 1999, Mday: 29, Month: 3, Second: 1,
				}.Timestamp(): types.TimeStruct{
					Year: 1999, Mday: 31, Month: 3,
				}.Timestamp(),
				// should skip 31.04
				types.TimeStruct{
					Year: 1999, Mday: 29, Month: 4, Second: 1,
				}.Timestamp(): types.TimeStruct{
					Year: 2000, Mday: 29, Month: 3,
				}.Timestamp(),
			},
		},
		{
			spec: "0 0 1 * 1", // At 00:00 on day-of-month 1 and on Monday
			roundUps: map[int64]int64{
				types.TimeStruct{
					Year: 1999, Mday: 1, Month: 4,
				}.Timestamp(): types.TimeStruct{
					Year: 1999, Mday: 1, Month: 11,
				}.Timestamp(),
			},
		},
		{
			spec: "0 0 1 1 1", // At 00:00 on day-of-month 1 and on Monday
			roundUps: map[int64]int64{
				0: types.TimeStruct{
					Year: 1973, Mday: 1, Month: 1,
				}.Timestamp(),
				types.TimeStruct{
					Year: 1973, Mday: 1, Month: 1, Second: 1,
				}.Timestamp(): types.TimeStruct{
					Year: 1979, Mday: 1, Month: 1,
				}.Timestamp(),
			},
		},
	}
	for _, c := range cases {
		spec, err := types.ParseCronSpec(c.spec)
		require.NoError(t, err)
		compiled := spec.Compile()
		require.True(t, compiled.IsValid())
		for k, v := range c.roundUps {
			require.Equal(t, v, compiled.RoundUp(k, 0))
		}
	}
}

func parseAndValidate(s string) (types.CompiledCronSpec, error) {
	spec, err := types.ParseCronSpec(s)
	if err != nil {
		return types.CompiledCronSpec{}, err
	}
	compiled := spec.Compile()
	if !compiled.IsValid() {
		return types.CompiledCronSpec{}, errors.New("invalid cron spec")
	}
	return compiled, nil
}

func TestInvalidCronSpecs(t *testing.T) {
	// bad format
	_, err := parseAndValidate("")
	require.Error(t, err)
	_, err = parseAndValidate("* * * *")
	require.Error(t, err)
	_, err = parseAndValidate("1,3-5-1- * * * *")
	require.Error(t, err)

	// out of range
	_, err = parseAndValidate("60 * * * *")
	require.Error(t, err)
	_, err = parseAndValidate("* 24 * * *")
	require.Error(t, err)
	_, err = parseAndValidate("* * 0 * *")
	require.Error(t, err)
	_, err = parseAndValidate("* * 32 * *")
	require.Error(t, err)
	_, err = parseAndValidate("* * * 0 *")
	require.Error(t, err)
	_, err = parseAndValidate("* * * 13 *")
	require.Error(t, err)
	_, err = parseAndValidate("* * * * 7")
	require.Error(t, err)

	// non exist month day
	_, err = parseAndValidate("* * 30 2 *")
	require.Error(t, err)
	_, err = parseAndValidate("* * 31 4 *")
	require.Error(t, err)
	_, err = parseAndValidate("* * 31 2,4,6,9,11 *")
	require.Error(t, err)
}

func TestValidCronSpecs(t *testing.T) {
	_, err := parseAndValidate("@yearly")
	require.NoError(t, err)
	_, err = parseAndValidate("@monthly")
	require.NoError(t, err)
	_, err = parseAndValidate("@daily")
	require.NoError(t, err)
	_, err = parseAndValidate("@weekly")
	require.NoError(t, err)
	_, err = parseAndValidate("@hourly")
	require.NoError(t, err)
	_, err = parseAndValidate("1-59-2 * * * *")
	require.NoError(t, err)
}

func TestCountPeriods(t *testing.T) {
	spec, err := types.ParseCronSpec("@yearly")
	require.NoError(t, err)
	beginTime := int64(0)        // 1970.1.1
	endTime := int64(1577836800) // 2020.1.1
	require.Equal(t, uint64(49), spec.Compile().CountPeriods(beginTime, endTime, 0, func() {}))
}
