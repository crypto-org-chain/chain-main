package types_test

import (
	"math/rand"
	"testing"
	"testing/quick"
	"time"

	"github.com/crypto-org-chain/chain-main/v1/x/subscription/types"
	"github.com/stretchr/testify/require"
)

// testTimestamp test TimeStruct against standard library
func TimestampTest(t *testing.T, ts int64) {
	t1 := time.Unix(ts, 0)
	t2 := types.TimeStruct{
		Year:   t1.Year(),
		Month:  int(t1.Month()),
		Mday:   t1.Day(),
		Yday:   t1.YearDay(),
		Wday:   int(t1.Weekday()),
		Hour:   t1.Hour(),
		Minute: t1.Minute(),
		Second: t1.Second(),
	}
	t3 := types.SecsToTM(ts, 0)
	require.Equal(t, t2, t3)
	require.Equal(t, ts, t3.Timestamp())
}

func TestSecsToTM(t *testing.T) {
	time.Local = time.UTC
	TimestampTest(t, int64(-951782400))
	TimestampTest(t, int64(-13574534400))
	TimestampTest(t, int64(0))
	TimestampTest(t, int64(951782400))
	TimestampTest(t, int64(13574534400))
}

func TestSecsToTMRandom(t *testing.T) {
	time.Local = time.UTC
	var i int64
	for i = 0; i < 10000; i++ {
		TimestampTest(t, rand.Int63()) //nolint: gosec
	}
}

func TestWeekday(t *testing.T) {
	cases := []struct {
		year, month, day int
	}{
		{1970, 1, 1},
		{2000, 2, 29},
		{1000, 2, 29},
	}
	for _, c := range cases {
		weekday := int(time.Date(c.year, time.Month(c.month), c.day, 0, 0, 0, 0, time.UTC).Weekday())
		require.Equal(t, weekday, types.Weekday(c.year, c.month, c.day))
	}
}

func TestTimezone(t *testing.T) {
	tm := types.SecsToTM(1614312703, 0) // UTC
	require.Equal(t, types.TimeStruct{
		Year:   2021,
		Month:  2,
		Mday:   26,
		Wday:   5,
		Yday:   57,
		Hour:   4,
		Minute: 11,
		Second: 43,
	}, tm)
	tm = types.SecsToTM(1614312703, 8*3600) // UTC+8
	require.Equal(t, types.TimeStruct{
		Year:   2021,
		Month:  2,
		Mday:   26,
		Wday:   5,
		Yday:   57,
		Hour:   12,
		Minute: 11,
		Second: 43,
	}, tm)
}

func TestRoundstrip(t *testing.T) {
	time.Local = time.UTC
	f := func(ts int64, tzoffset int32) bool {
		if ts+int64(tzoffset) < -62135596800 {
			// We don't handle negative year
			return true
		}
		t1 := time.Unix(ts+int64(tzoffset), 0)
		t2 := types.TimeStruct{
			Year:   t1.Year(),
			Month:  int(t1.Month()),
			Mday:   t1.Day(),
			Yday:   t1.YearDay(),
			Wday:   int(t1.Weekday()),
			Hour:   t1.Hour(),
			Minute: t1.Minute(),
			Second: t1.Second(),
		}
		t3 := types.SecsToTM(ts, tzoffset)
		return t2 == t3 && ts+int64(tzoffset) == t3.Timestamp()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
