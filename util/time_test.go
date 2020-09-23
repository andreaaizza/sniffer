package util

import (
	"testing"
	"time"
)

func TestTimeBuilder(t *testing.T) {
	t0 := time.Now()
	ts0 := TimestampBuilder(t0)
	t1 := TimeBuilder(ts0)
	ts1 := TimestampBuilder(t1)
	if t0.UnixNano() != t1.UnixNano() {
		t.Errorf("time builder does not match t=%v, ts=%v, t1=%v, ts1=%v", t0, ts0, t1, ts1)
	}
}
