package persistence

import "testing"

var validDates = []struct {
	date        string
	granularity Granularity
}{
	{
		"2018",
		Year,
	},
	{
		"2018-04",
		Month,
	},
	{
		"2018-W16",
		Week,
	},
	{
		"2018-04-20",
		Day,
	},
	{
		"2018-04-20T15",
		Hour,
	},
}

func TestParseISO8601(t *testing.T) {
	for i, date := range validDates {
		_, gr, err := ParseISO8601(date.date)
		if err != nil {
			t.Errorf("Error while parsing valid date #%d: %s", i+1, err.Error())
			continue
		}

		if gr != date.granularity {
			t.Errorf("Granularity of the date #%d is wrong! Got %d (expected %d)",
				i+1, gr, date.granularity)
			continue
		}
	}
}
