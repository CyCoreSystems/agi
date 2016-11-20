package agi

import (
	"strconv"
	"time"
)

func toMSec(dur time.Duration) string {
	return strconv.Itoa(int(1000 * dur.Seconds()))
}

func toSec(dur time.Duration) string {
	return strconv.Itoa(int(dur.Seconds()))
}

func toEpoch(when time.Time) string {
	return strconv.FormatInt(when.Unix(), 10)
}
