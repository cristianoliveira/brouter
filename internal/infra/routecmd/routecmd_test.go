package routecmd

import (
	"testing"
	"time"
)

func TestFreezeClockContract(t *testing.T) {
	c := func() time.Time { return time.Unix(1700000000, 0).UTC() }
	u := frozenClock(c)
	if u.Epoch != 1700000000 {
		t.Errorf("epoch = %d", u.Epoch)
	}
	if u.Year != 2023 || u.Month != 11 || u.Day != 14 {
		t.Errorf("utc date = %d-%d-%d", u.Year, u.Month, u.Day)
	}
	if u.Hour != 22 || u.Min != 13 || u.Sec != 20 {
		t.Errorf("utc time = %d:%d:%d", u.Hour, u.Min, u.Sec)
	}
}
