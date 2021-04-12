package types_test

import (
	"testing"

	"github.com/crypto-org-chain/chain-main/v2/x/subscription/types"
)

func BenchmarkRoundUpYearly(b *testing.B) {
	spec, err := types.ParseCronSpec("@yearly")
	if err != nil {
		panic(err)
	}
	compiled := spec.Compile()
	start := types.TimeStruct{
		Year: 1999, Mday: 1, Month: 1, Second: 1,
	}.Timestamp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.RoundUp(start, 0)
	}
}

func BenchmarkRoundUpMonthly(b *testing.B) {
	spec, err := types.ParseCronSpec("@monthly")
	if err != nil {
		panic(err)
	}
	compiled := spec.Compile()
	start := types.TimeStruct{
		Year: 2000, Mday: 2, Month: 1, Second: 1,
	}.Timestamp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.RoundUp(start, 0)
	}
}

func BenchmarkRoundUpWeekly(b *testing.B) {
	spec, err := types.ParseCronSpec("@weekly")
	if err != nil {
		panic(err)
	}
	compiled := spec.Compile()
	start := types.TimeStruct{
		Year: 2021, Mday: 4, Month: 5, Second: 1,
	}.Timestamp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.RoundUp(start, 0)
	}
}

func BenchmarkRoundUpLeapYear(b *testing.B) {
	spec, err := types.ParseCronSpec("0 0 29 2 *")
	if err != nil {
		panic(err)
	}
	compiled := spec.Compile()
	start := types.TimeStruct{
		Year: 2000, Mday: 1, Month: 3,
	}.Timestamp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.RoundUp(start, 0)
	}
}

func BenchmarkRoundUpOverlap(b *testing.B) {
	spec, err := types.ParseCronSpec("0 0 1 1 1")
	if err != nil {
		panic(err)
	}
	compiled := spec.Compile()
	start := types.TimeStruct{
		Year: 1973, Mday: 1, Month: 1, Second: 1,
	}.Timestamp()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.RoundUp(start, 0)
	}
}

func BenchmarkCompile(b *testing.B) {
	spec, err := types.ParseCronSpec("* * * * *")
	if err != nil {
		panic(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spec.Compile()
	}
}

func BenchmarkCountPeriods(b *testing.B) {
	spec, err := types.ParseCronSpec("@hourly")
	if err != nil {
		panic(err)
	}
	compiled := spec.Compile()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.CountPeriods(0, 31536000, 0) // period of a year
	}
}
