package internal

import (
	"os"
	"testing"
)

// Go reports a writable Windows file as 0666 and a folder as 0777, and NTFS has
// no group or others to back that up. A Linux client reads those bits as world
// writable, which programs that check, like MySQL with its config files, refuse.
func TestModeFromOSDropsGroupAndOtherWrite(t *testing.T) {
	cases := []struct {
		in   os.FileMode
		want uint32
	}{
		{0o666, ModeRegular | 0o644},
		{0o444, ModeRegular | 0o444},
		{os.ModeDir | 0o777, ModeDirectory | 0o755},
		{os.ModeSymlink | 0o777, ModeSymlink | 0o777},
	}
	for _, c := range cases {
		if got := modeFromOS(c.in); got != c.want {
			t.Errorf("modeFromOS(%v) = %o, want %o", c.in, got, c.want)
		}
	}
}
