package pm2

import "testing"

func TestFormatMemory(t *testing.T) {
	cases := []struct {
		mem  int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1 KB"},
		{1536, "1.5 KB"},
		{10 * 1024 * 1024, "10 MB"},
		{4089446, "3.9 MB"},
		{2 << 30, "2 GB"},
		{int64(1.5 * float64(1<<30)), "1.5 GB"},
	}
	for _, c := range cases {
		p := Process{Monit: Monit{Memory: c.mem}}
		if got := p.FormatMemory(); got != c.want {
			t.Errorf("FormatMemory(%d) = %q, want %q", c.mem, got, c.want)
		}
	}
}
