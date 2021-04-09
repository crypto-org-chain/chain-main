package types

import (
	"errors"
	"math"
	"strconv"
	"strings"

	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// CompiledCronSpec store bitset form of cron spec
type CompiledCronSpec struct {
	Minute CompiledCronItem
	Hour   CompiledCronItem
	Mday   CompiledCronItem
	Month  CompiledCronItem
	Wday   CompiledCronItem
}

type CompiledCronItem struct {
	set BitSet
	Min uint32
	Max uint32
}

func NewCompileCronItem() CompiledCronItem {
	return CompiledCronItem{
		Min: math.MaxUint32,
		Max: 0,
	}
}

func (item *CompiledCronItem) Merge(other CompiledCronItem) {
	item.set.InPlaceUnion(other.set)
	if other.Min < item.Min {
		item.Min = other.Min
	}
	if other.Max > item.Max {
		item.Max = other.Max
	}
}

func (item CompiledCronItem) Empty() bool {
	return item.Min > item.Max
}

func (item *CompiledCronItem) Set(value uint32) error {
	err := item.set.Set(uint(value))
	if err != nil {
		return err
	}
	if value < item.Min {
		item.Min = value
	}
	if value > item.Max {
		item.Max = value
	}
	return nil
}

func (item CompiledCronItem) Test(value uint32) (bool, error) {
	return item.set.Test(uint(value))
}

func (spec CronSpec) Compile() (result CompiledCronSpec) {
	result.Minute = NewCompileCronItem()
	for _, spec := range spec.Minute {
		result.Minute.Merge(spec.Compile(0, 59))
	}

	result.Hour = NewCompileCronItem()
	for _, spec := range spec.Hour {
		result.Hour.Merge(spec.Compile(0, 23))
	}

	result.Mday = NewCompileCronItem()
	for _, spec := range spec.Mday {
		result.Mday.Merge(spec.Compile(1, 31))
	}

	result.Month = NewCompileCronItem()
	for _, spec := range spec.Month {
		result.Month.Merge(spec.Compile(1, 12))
	}

	result.Wday = NewCompileCronItem()
	for _, spec := range spec.Wday {
		result.Wday.Merge(spec.Compile(0, 6))
	}

	return
}

func (spec CompiledCronSpec) IsValid() bool {
	if spec.Minute.Empty() || spec.Hour.Empty() || spec.Mday.Empty() ||
		spec.Month.Empty() || spec.Wday.Empty() {
		return false
	}
	// Check range of values
	if spec.Minute.Max > 59 {
		return false
	}
	if spec.Hour.Max > 23 {
		return false
	}
	if spec.Mday.Min == 0 || spec.Mday.Max > 31 {
		return false
	}
	if spec.Wday.Max > 6 {
		return false
	}
	if spec.Month.Min == 0 || spec.Month.Max > 12 {
		return false
	}

	// Check non-exists month days
	validDays := NewBitSet()
	for i, e := spec.Month.set.NextSet(0); e; i, e = spec.Month.set.NextSet(i + 1) {
		validDays.InPlaceUnion(DaysInMonthSet[i-1])
	}
	return spec.Mday.set.Intersection(validDays).Len() != 0
}

func (item CronItem) Compile(min uint32, max uint32) CompiledCronItem {
	result := NewCompileCronItem()
	switch spec := item.GetSpec().(type) {
	case nil:
		for i := min; i <= max; i++ {
			err := result.Set(i)
			if err != nil {
				panic(err)
			}
		}
	case *CronItem_Value:
		err := result.Set(spec.Value)
		if err != nil {
			panic(err)
		}
	case *CronItem_Range:
		for i := spec.Range.Start; i <= spec.Range.Stop; i += spec.Range.Step {
			err := result.Set(i)
			if err != nil {
				panic(err)
			}
		}
	default:
		panic("impossible")
	}
	return result
}

func yearlyCronSpec() CronSpec {
	return CronSpec{
		Minute: []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Hour:   []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Mday:   []CronItem{{Spec: &CronItem_Value{Value: 1}}},
		Month:  []CronItem{{Spec: &CronItem_Value{Value: 1}}},
		Wday:   []CronItem{{}},
	}
}

func monthlyCronSpec() CronSpec {
	return CronSpec{
		Minute: []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Hour:   []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Mday:   []CronItem{{Spec: &CronItem_Value{Value: 1}}},
		Month:  []CronItem{{}},
		Wday:   []CronItem{{}},
	}
}

func weeklyCronSpec() CronSpec {
	return CronSpec{
		Minute: []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Hour:   []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Mday:   []CronItem{{}},
		Month:  []CronItem{{}},
		Wday:   []CronItem{{Spec: &CronItem_Value{Value: 1}}},
	}
}

func dailyCronSpec() CronSpec {
	return CronSpec{
		Minute: []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Hour:   []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Mday:   []CronItem{{}},
		Month:  []CronItem{{}},
		Wday:   []CronItem{{}},
	}
}

func hourlyCronSpec() CronSpec {
	return CronSpec{
		Minute: []CronItem{{Spec: &CronItem_Value{Value: 0}}},
		Hour:   []CronItem{{}},
		Mday:   []CronItem{{}},
		Month:  []CronItem{{}},
		Wday:   []CronItem{{}},
	}
}

// ParseCronSpec parse CronSpec from string
func ParseCronSpec(s string) (CronSpec, error) {
	switch s {
	case "@yearly":
		return yearlyCronSpec(), nil
	case "@monthly":
		return monthlyCronSpec(), nil
	case "@weekly":
		return weeklyCronSpec(), nil
	case "@daily":
		return dailyCronSpec(), nil
	case "@hourly":
		return hourlyCronSpec(), nil
	}
	parts := strings.Split(s, " ")
	if len(parts) != 5 {
		return CronSpec{}, errors.New("invalid cron spec")
	}
	minute, err := parseItemSpec(parts[0])
	if err != nil {
		return CronSpec{}, err
	}
	hour, err := parseItemSpec(parts[1])
	if err != nil {
		return CronSpec{}, err
	}
	mday, err := parseItemSpec(parts[2])
	if err != nil {
		return CronSpec{}, err
	}
	month, err := parseItemSpec(parts[3])
	if err != nil {
		return CronSpec{}, err
	}
	wday, err := parseItemSpec(parts[4])
	if err != nil {
		return CronSpec{}, err
	}
	return CronSpec{
		Minute: minute,
		Hour:   hour,
		Mday:   mday,
		Month:  month,
		Wday:   wday,
	}, nil
}

func parseItemSpec(s string) ([]CronItem, error) {
	parts := strings.Split(s, ",")
	var items = []CronItem{}
	for _, part := range parts {
		item, err := parseItemSpecSingle(part)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, errors.New("empty item spec")
	}
	return items, nil
}

func parseItemSpecSingle(s string) (CronItem, error) {
	switch {
	case s == "*":
		return CronItem{}, nil
	case strings.Contains(s, "-"):
		parts := strings.Split(s, "-")
		if len(parts) == 2 || len(parts) == 3 {
			start, err := strconv.ParseUint(parts[0], 10, 32)
			if err != nil {
				return CronItem{}, err
			}
			stop, err := strconv.ParseUint(parts[1], 10, 32)
			if err != nil {
				return CronItem{}, err
			}
			var step uint64 = 1
			if len(parts) == 3 {
				step, err = strconv.ParseUint(parts[1], 10, 32)
				if err != nil {
					return CronItem{}, err
				}
			}
			return CronItem{Spec: &CronItem_Range{Range: &CronRange{Start: uint32(start), Stop: uint32(stop), Step: uint32(step)}}}, nil
		}
		return CronItem{}, errors.New("invalid range spec")
	default:
		value, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return CronItem{}, err
		}
		return CronItem{Spec: &CronItem_Value{Value: uint32(value)}}, nil
	}
}

/*RoundUp returns smallest timestamp matches spec and bigger or equal than ts

Process the most significant part first, which is the month.
If month don't match exactly, round up or wrap around:
- choose min values for day hour minute
- if wrapped, add a year
if month match exactly, proceeds to next part: day
- If day don't match exactly, round up or wrap around:
  - choose min values for hour and minute
  - if wrapped, add a month and try again
- If day match exactly, proceed to next part: hour
  - if hour don't match exactly, round up or wrap around:
    - choose min value for minute
    - if wrapped, add an hour and try again
  - if hour match exactly, proceed to next part: minute
*/
func (spec CompiledCronSpec) RoundUp(ts int64, tzoffset int32) int64 {
	var minute, hour, mday int

	// round up the seconds first
	seconds := ts % 60
	if seconds > 0 {
		ts += (60 - seconds)
	}
	tm := SecsToTM(ts, tzoffset)
	for {
		month := int(roundUpItem(spec.Month, uint32(tm.Month)))
		if month != tm.Month {
			if month < tm.Month {
				tm.Year++
			}
			mday = int(spec.Mday.Min)
			// validate month day
			if mday > MonthDays(tm.Year, month) {
				// invalid month day, try next month
				tm.Month = month + 1
				continue
			}
			tm.Month = month
			tm.Mday = mday
			tm.Hour = int(spec.Hour.Min)
			tm.Minute = int(spec.Minute.Min)
			goto Success
		}

		mday = int(roundUpItem(spec.Mday, uint32(tm.Mday)))
		if mday != tm.Mday {
			if mday < tm.Mday {
				tm.Mday = mday
				tm.Month++
				continue
			}
			if mday > MonthDays(tm.Year, tm.Month) {
				tm.Mday = 1
				tm.Month++
				continue
			}
			tm.Mday = mday
			tm.Hour = int(spec.Hour.Min)
			tm.Minute = int(spec.Minute.Min)
			goto Success
		}

		hour = int(roundUpItem(spec.Hour, uint32(tm.Hour)))
		if hour != tm.Hour {
			wrapped := hour < tm.Hour
			tm.Hour = hour
			tm.Minute = int(spec.Minute.Min)
			if wrapped {
				// wrapped, add a day and try again
				tm = SecsToTM(tm.Timestamp()+86400, tzoffset)
				continue
			}
			goto Success
		}

		minute = int(roundUpItem(spec.Minute, uint32(tm.Minute)))
		if minute != tm.Minute {
			wrapped := minute < tm.Minute
			tm.Minute = minute
			if wrapped {
				// wrapped, add an hour and try again
				tm = SecsToTM(tm.Timestamp()+3600, tzoffset)
				continue
			}
		}

	Success:
		// Check weekday
		result, error := spec.Wday.Test(uint32(Weekday(tm.Year, tm.Month, tm.Mday)))
		if error != nil {
			// that shouldn't happen, we need to crash the app
			err := sdkErrors.Wrap(error, "Wday.Test did panic with OutOfBound error")
			panic(err)
		}
		if !result {
			tm.Mday++
			tm.Hour = int(spec.Hour.Min)
			tm.Minute = int(spec.Minute.Min)
			continue
		}
		break
	}
	tm.Second = 0
	return tm.Timestamp()
}

// Return the smallest value matches specs that is bigger or equal than `v`, wrap around if necessary.
func roundUpItem(spec CompiledCronItem, v uint32) uint32 {
	result, found := spec.set.NextSet(uint(v))
	if !found {
		return spec.Min
	}
	return uint32(result)
}

// CountPeriods counts how many datetimes between beginTime expirationTime matches the cron spec.
// beginTime is not inclusive.
func (spec CompiledCronSpec) CountPeriods(beginTime, expirationTime int64, tzoffset int32) (count uint64) {
	count = 0
	for {
		beginTime = spec.RoundUp(beginTime+1, tzoffset)
		if beginTime >= expirationTime {
			break
		}
		count++
	}
	return
}
