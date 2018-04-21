package persistence

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var yearRE  = regexp.MustCompile(`^(\d{4})$`)
var monthRE = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
var weekRE  = regexp.MustCompile(`^(\d{4})-W(\d{2})$`)
var dayRE   = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
var hourRE  = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})T(\d{2})$`)

type Granularity int
const (
	Year Granularity = iota
	Month
	Week
	Day
	Hour
)

// ParseISO8601 is **not** a function to parse all and every kind of valid ISO 8601
// date, nor it's intended to be, since we don't need that.
func ParseISO8601(s string) (*time.Time, Granularity, error) {
	if matches := yearRE.FindStringSubmatch(s); len(matches) != 0 {
		year, err := parseYear(matches[1])
		if err != nil {
			return nil, -1, err
		}
		t := time.Date(year, time.December, daysOfMonth(time.December, year), 23, 59, 59, 0, time.UTC)
		return &t, Year, nil
	}

	if matches := monthRE.FindStringSubmatch(s); len(matches) != 0 {
		month, err := parseMonth(matches[2])
		year, err := parseYear(matches[1])
		if err != nil {
			return nil, -1, err
		}
		t := time.Date(year, month, 31, 23, 59, 59, 0, time.UTC)
		return &t, Month, nil
	}

	if matches := weekRE.FindStringSubmatch(s); len(matches) != 0 {
		week, err := parseWeek(matches[2])
		year, err := parseYear(matches[1])
		if err != nil {
			return nil, -1, err
		}
		t := time.Date(year, time.January, week * 7, 23, 59, 59, 0, time.UTC)
		return &t, Week, nil
	}

	if matches := dayRE.FindStringSubmatch(s); len(matches) != 0 {
		month, err := parseMonth(matches[2])
		year, err := parseYear(matches[1])
		if err != nil {
			return nil, -1, err
		}
		day, err := parseDay(matches[3], daysOfMonth(month, year))
		if err != nil {
			return nil, -1, err
		}
		t := time.Date(year, month, day, 23, 59, 59, 0, time.UTC)
		return &t, Day, nil
	}

	if matches := hourRE.FindStringSubmatch(s); len(matches) != 0 {
		month, err := parseMonth(matches[2])
		year, err := parseYear(matches[1])
		if err != nil {
			return nil, -1, err
		}
		hour, err := parseHour(matches[4])
		day, err := parseDay(matches[3], daysOfMonth(month, year))
		if err != nil {
			return nil, -1, err
		}
		t := time.Date(year, month, day, hour, 59, 59, 0, time.UTC)
		return &t, Hour, nil
	}

	return nil, -1, fmt.Errorf("string does not match any formats")
}

func daysOfMonth(month time.Month, year int) int {
	switch month {
	case time.January:
		return 31
	case time.February:
		if isLeap(year) {
			return 29
		} else {
			return 28
		}
	case time.March:
		return 31
	case time.April:
		return 30
	case time.May:
		return 31
	case time.June:
		return 30
	case time.July:
		return 31
	case time.August:
		return 31
	case time.September:
		return 30
	case time.October:
		return 31
	case time.November:
		return 30
	case time.December:
		return 31
	default:
		panic("invalid month!")
	}
}

func isLeap(year int) bool {
	 if year % 4 != 0 {
	 	return false
	 } else if year % 100 != 0 {
	 	return true
	 } else if year % 400 != 0 {
	 	return false
	 } else {
	 	return true
	 }
}

func atoi(s string) int {
	i, e := strconv.Atoi(s)
	if e != nil {
		// panic on error since atoi() will be called only after we parse it with regex
		// (hopefully `\d`!)
		panic(e.Error())
	}
	return i
}

func parseYear(s string) (int, error) {
	year := atoi(s)
	if year <= 1583 {
		return 0, fmt.Errorf("years before 1583 are not allowed")
	}
	return year, nil
}

func parseMonth(s string) (time.Month, error) {
	month := atoi(s)
	if month <= 0  || month >= 13 {
		return time.Month(-1), fmt.Errorf("month is not in range [01, 12]")
	}
	return time.Month(month), nil
}

func parseWeek(s string) (int, error) {
	week := atoi(s)
	if week <= 0 || week >= 54 {
		return -1, fmt.Errorf("week is not in range [01, 53]")
	}
	return week, nil
}

func parseDay(s string, max int) (int, error) {
	day := atoi(s)
	if day <= 0 || day > max {
		return -1, fmt.Errorf("day is not in range [01, %d]", max)
	}
	return day, nil
}

func parseHour(s string) (int, error) {
	hour := atoi(s)
	if hour <= -1 || hour >= 25 {
		return -1, fmt.Errorf("hour is not in range [00, 24]")
	}
	return hour, nil
}
