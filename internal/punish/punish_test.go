package punish

import "testing"

func TestRemaining(t *testing.T) {
	cases := []struct {
		left    int64
		d, h, i string
	}{
		{59, "0", "00", "00"},
		{3661, "0", "01", "01"},
		{86400 + 7200 + 300, "01", "02", "05"},
		{40*86400 + 60, "40", "00", "01"},
		{-5, "0", "00", "00"},
	}
	for _, c := range cases {
		d, h, i := Remaining(c.left)
		if d != c.d || h != c.h || i != c.i {
			t.Errorf("%d: got %s %s %s", c.left, d, h, i)
		}
	}
}
