package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cron is a routine's schedule: cron's five fields (minute, hour, day of month, month,
// day of week). Each field is *, a number, a range a-b, a step */n or a-b/n, or a list
// of those joined by commas. Day of week is 0-7 (0 and 7 are Sunday). As in cron, when
// both day fields are restricted (neither starts with *), a day matches either one.
// Names (MON, JAN) and @daily-style shortcuts are not read.
type cron struct {
	minute, hour, dom, month, dow uint64 // bit n set: n matches
	domAny, dowAny                bool
}

var cronFields = []struct {
	name     string
	min, max int
}{{"minute", 0, 59}, {"hour", 0, 23}, {"day of month", 1, 31}, {"month", 1, 12}, {"day of week", 0, 7}}

func parseCron(expr string) (*cron, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("%q is not five fields", expr)
	}
	var bits [5]uint64
	for i, p := range parts {
		b, err := cronField(p, cronFields[i].min, cronFields[i].max)
		if err != nil {
			return nil, fmt.Errorf("%s field %q: %v", cronFields[i].name, p, err)
		}
		bits[i] = b
	}
	if bits[4]&(1<<7) != 0 {
		bits[4] |= 1 // 7 is Sunday too
	}
	return &cron{minute: bits[0], hour: bits[1], dom: bits[2], month: bits[3], dow: bits[4],
		domAny: strings.HasPrefix(parts[2], "*"), dowAny: strings.HasPrefix(parts[4], "*")}, nil
}

func cronField(s string, lo, hi int) (uint64, error) {
	var bits uint64
	for _, item := range strings.Split(s, ",") {
		rng, step, hasStep := strings.Cut(item, "/")
		n := 1
		if hasStep {
			var err error
			if n, err = strconv.Atoi(step); err != nil || n < 1 {
				return 0, fmt.Errorf("step %q", step)
			}
		}
		from, to := lo, hi
		if rng != "*" {
			a, b, isRange := strings.Cut(rng, "-")
			var err error
			if from, err = cronNumber(a, lo, hi); err != nil {
				return 0, err
			}
			to = from
			if isRange {
				if to, err = cronNumber(b, lo, hi); err != nil {
					return 0, err
				}
				if to < from {
					return 0, fmt.Errorf("range %q goes backwards", rng)
				}
			} else if hasStep {
				return 0, fmt.Errorf("a step needs * or a range: %q", item)
			}
		}
		for v := from; v <= to; v += n {
			bits |= 1 << v
		}
	}
	return bits, nil
}

func cronNumber(s string, lo, hi int) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil || strings.ContainsAny(s, "+-") {
		return 0, fmt.Errorf("%q is not a number", s)
	}
	if v < lo || v > hi {
		return 0, fmt.Errorf("%d is not in %d-%d", v, lo, hi)
	}
	return v, nil
}

func (c *cron) dayMatches(t time.Time) bool {
	dom, dow := c.dom&(1<<t.Day()) != 0, c.dow&(1<<int(t.Weekday())) != 0
	if c.domAny || c.dowAny {
		return dom && dow
	}
	return dom || dow
}

// cronHorizon is how far next looks; a schedule with no time in it never fires (Feb 30).
const cronHorizon = 5 * 366 * 24 * time.Hour

// next is the first time after `after` that the schedule names, read in loc.
func (c *cron) next(after time.Time, loc *time.Location) (time.Time, bool) {
	t := after.In(loc).Truncate(time.Minute).Add(time.Minute)
	end := after.Add(cronHorizon)
	for t.Before(end) {
		y, mo, d := t.Date()
		var n time.Time
		switch {
		case c.month&(1<<int(mo)) == 0:
			n = time.Date(y, mo+1, 1, 0, 0, 0, 0, loc)
		case !c.dayMatches(t):
			n = time.Date(y, mo, d+1, 0, 0, 0, 0, loc)
		case c.hour&(1<<t.Hour()) == 0:
			n = time.Date(y, mo, d, t.Hour()+1, 0, 0, 0, loc)
		case c.minute&(1<<t.Minute()) == 0:
			n = t.Add(time.Minute)
		default:
			return t, true
		}
		if !n.After(t) { // a clock change can map a local time back; always move on
			n = t.Add(time.Minute)
		}
		t = n
	}
	return time.Time{}, false
}
