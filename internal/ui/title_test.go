package ui

import (
	"testing"

	"github.com/DillonBarker/pm2ui/internal/pm2"
)

func proc(name, status string) pm2.Process {
	return pm2.Process{Name: name, PM2Env: pm2.PM2Env{Status: status}}
}

func TestWindowTitle(t *testing.T) {
	tests := []struct {
		name  string
		procs []pm2.Process
		want  string
	}{
		{
			name:  "no processes",
			procs: nil,
			want:  "pm2ui",
		},
		{
			name:  "all online",
			procs: []pm2.Process{proc("api", pm2.StatusOnline), proc("web", pm2.StatusOnline)},
			want:  "pm2ui ●2",
		},
		{
			name: "errored named after count",
			procs: []pm2.Process{
				proc("api", pm2.StatusErrored),
				proc("web", pm2.StatusOnline),
				proc("worker", pm2.StatusErrored),
			},
			want: "pm2ui ●1 ✖api,worker",
		},
		{
			name:  "stopped uses pause marker",
			procs: []pm2.Process{proc("api", pm2.StatusOnline), proc("web", pm2.StatusStopped)},
			want:  "pm2ui ●1 ⏸web",
		},
		{
			name: "errored marker wins over stopped",
			procs: []pm2.Process{
				proc("api", pm2.StatusStopped),
				proc("web", pm2.StatusErrored),
			},
			want: "pm2ui ●0 ✖api,web",
		},
		{
			name: "launching counts as down",
			procs: []pm2.Process{
				proc("api", pm2.StatusOnline),
				proc("web", pm2.StatusLaunching),
			},
			want: "pm2ui ●1 ⏸web",
		},
		{
			name: "overflowing names collapse to tally",
			procs: []pm2.Process{
				proc("billing", pm2.StatusErrored),
				proc("payments", pm2.StatusErrored),
				proc("shipping", pm2.StatusErrored),
				proc("checkout", pm2.StatusErrored),
				proc("catalogue", pm2.StatusErrored),
			},
			want: "pm2ui ●0 ✖billing,payments,+3",
		},
		{
			name:  "single long name is kept whole",
			procs: []pm2.Process{proc("a-very-long-service-name-indeed", pm2.StatusErrored)},
			want:  "pm2ui ●0 ✖a-very-long-service-name-indeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WindowTitle(tt.procs); got != tt.want {
				t.Errorf("WindowTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWindowTitleNameBudget(t *testing.T) {
	procs := []pm2.Process{
		proc("aaaaaaaaaa", pm2.StatusErrored),
		proc("bbbbbbbbbb", pm2.StatusErrored),
		proc("cccccccccc", pm2.StatusErrored),
	}
	got := WindowTitle(procs)
	names := got[len("pm2ui ●0 ✖"):]
	if len([]rune(names)) > titleNameBudget {
		t.Errorf("name section %q is %d runes, over budget %d", names, len([]rune(names)), titleNameBudget)
	}
}
