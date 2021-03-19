package types

// port from https://github.com/esmil/musl/blob/master/src/time/__secs_to_tm.c

const (
	leaPoch        = 946684800 + 86400*(31+29) // 2000-03-01
	daysPer400Y    = 365*400 + 97
	daysPer100Y    = 365*100 + 24
	daysPer4Y      = 365*4 + 1
	daysBefore1970 = 719162
)

var (
	// start at March
	daysInMonth = [...]int{31, 30, 31, 30, 31, 31, 30, 31, 30, 31, 31, 29}
	// start at Jan
	daysInMonthNoLeap = [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	daysInMonthLeap   = [...]int{31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	// start at January
	daysBeforeMonth = [...]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	// DaysInMonthSet start at Jan
	DaysInMonthSet []BitSet
)

func init() {
	for _, days := range daysInMonthLeap {
		bs := NewBitSet()
		for i := 1; i <= days; i++ {
			err := bs.Set(uint(i))
			if err != nil {
				panic(err)
			}
		}
		DaysInMonthSet = append(DaysInMonthSet, bs)
	}
}

// TimeStruct is struct tm
type TimeStruct struct {
	Year   int
	Month  int // 1 - 12
	Mday   int // 1 -
	Wday   int // 0 - 6, start from Sunday
	Yday   int // 1 -
	Hour   int // 0 - 23
	Minute int // 0 - 59
	Second int // 0 - 59
}

// Timestamp calculate utc timestamp from time struct
func (tm TimeStruct) Timestamp() int64 {
	days := int64(DateToOrdinal(tm.Year, tm.Month, tm.Mday) - daysBefore1970)
	return days*86400 + int64(tm.Hour)*3600 + int64(tm.Minute)*60 + int64(tm.Second)
}

// SecsToTM converts timestamp to TimeStruct
func SecsToTM(t int64, tzoffset int32) TimeStruct {
	// add timezone offset
	t += int64(tzoffset)

	secs := t - leaPoch
	days, remsecs := divmod(secs, 86400)

	_, wDay := divmod(3+days, 7)

	qcCycles, remDays := divmod(days, daysPer400Y)

	cCycles := remDays / daysPer100Y
	if cCycles == 4 {
		cCycles--
	}
	remDays -= cCycles * daysPer100Y

	qCycles := remDays / daysPer4Y
	if qCycles == 25 {
		// TODO I think this path is never reached
		qCycles--
	}
	remDays -= qCycles * daysPer4Y

	remYears := remDays / 365
	if remYears == 4 {
		remYears--
	}
	remDays -= remYears * 365

	isLeap := remYears == 0 && (qCycles > 0 || cCycles == 0)
	var leap int64 = 0
	if isLeap {
		leap = 1
	}

	yDay := (remDays + int64(31) + int64(28) + leap) % (int64(365) + leap)
	years := remYears + 4*qCycles + 100*cCycles + 400*qcCycles

	var months int
	var n int
	for months, n = range daysInMonth {
		if remDays < int64(n) {
			break
		}
		remDays -= int64(n)
	}
	months += 3

	if months > 12 {
		months -= 12
		years++
	}

	return TimeStruct{
		Year:   int(years + 2000),
		Month:  months,
		Mday:   int(remDays + 1),
		Wday:   int(wDay),
		Yday:   int(yDay + 1),
		Hour:   int(remsecs / 3600),
		Minute: int((remsecs / 60) % 60),
		Second: int(remsecs % 60),
	}
}

// isLeapYear returns if the year is leap year
func isLeapYear(year int) bool {
	return positiveMod(year, 4) == 0 && (positiveMod(year, 100) != 0 || positiveMod(year, 400) == 0)
}

// MonthDays calculates how many days in a month
func MonthDays(year int, month int) int {
	leap := 0
	if isLeapYear(year) && month == 2 {
		leap = 1
	}
	return daysInMonthNoLeap[month-1] + leap
}

func DateToOrdinal(year, month, day int) int {
	lastYear := year - 1
	daysBeforeYear := lastYear*365 + lastYear/4 - lastYear/100 + lastYear/400
	daysBeforeMonth := daysBeforeMonth[month-1]
	if month > 2 && isLeapYear(year) {
		daysBeforeMonth++
	}
	return daysBeforeYear + daysBeforeMonth + day - 1
}

// Weekday return the weekday(0-6) of the date
// Panic if input is invalid
func Weekday(year, month, day int) int {
	if year < 1 || month < 1 || day < 1 {
		panic("invalid date input")
	}
	return (DateToOrdinal(year, month, day) + 1) % 7
}

func divmod(numerator, denominator int64) (quotient, remainder int64) {
	quotient = numerator / denominator // integer division, decimals are truncated
	remainder = numerator % denominator
	if remainder < 0 {
		remainder += denominator
		quotient--
	}
	return
}

func positiveMod(numerator, denominator int) (remainder int) {
	remainder = numerator % denominator
	if remainder < 0 {
		remainder += denominator
	}
	return
}
