package pm2

import "time"

// Process represents a PM2 process from `pm2 jlist` output.
type Process struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	PM2ID      int     `json:"pm_id"`
	Monit      Monit   `json:"monit"`
	PM2Env     PM2Env  `json:"pm2_env"`
}

// Monit holds resource usage metrics.
type Monit struct {
	Memory int64   `json:"memory"`
	CPU    float64 `json:"cpu"`
}

// PM2Env holds the PM2 environment configuration for a process.
type PM2Env struct {
	Status        string `json:"status"`
	PMUptime      int64  `json:"pm_uptime"`
	RestartTime   int    `json:"restart_time"`
	PMOutLogPath  string `json:"pm_out_log_path"`
	PMErrLogPath  string `json:"pm_err_log_path"`
	ExecMode      string `json:"exec_mode"`
	NodeVersion   string `json:"node_version"`
}

// StatusOnline is the PM2 status for a running process.
const (
	StatusOnline   = "online"
	StatusStopped  = "stopped"
	StatusErrored  = "errored"
	StatusStopping = "stopping"
	StatusLaunching = "launching"
)

// Uptime returns the duration since the process was started.
func (p *Process) Uptime() time.Duration {
	if p.PM2Env.PMUptime == 0 {
		return 0
	}
	started := time.UnixMilli(p.PM2Env.PMUptime)
	return time.Since(started).Truncate(time.Second)
}

// FormatMemory returns memory usage in a human-readable format.
func (p *Process) FormatMemory() string {
	mem := p.Monit.Memory
	switch {
	case mem >= 1<<30:
		return formatFloat(float64(mem)/float64(1<<30)) + " GB"
	case mem >= 1<<20:
		return formatFloat(float64(mem)/float64(1<<20)) + " MB"
	case mem >= 1<<10:
		return formatFloat(float64(mem)/float64(1<<10)) + " KB"
	default:
		return formatFloat(float64(mem)) + " B"
	}
}

// FormatUptime returns uptime in a human-readable format.
func (p *Process) FormatUptime() string {
	if p.PM2Env.Status != StatusOnline {
		return "-"
	}
	d := p.Uptime()
	if d <= 0 {
		return "-"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	switch {
	case days > 0:
		return formatInt(days) + "d" + formatInt(hours) + "h"
	case hours > 0:
		return formatInt(hours) + "h" + formatInt(mins) + "m"
	default:
		secs := int(d.Seconds()) % 60
		return formatInt(mins) + "m" + formatInt(secs) + "s"
	}
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return formatInt(int(f))
	}
	// one decimal place
	return trimTrailingZeros(f)
}

func trimTrailingZeros(f float64) string {
	s := ""
	whole := int(f)
	frac := int((f - float64(whole)) * 10)
	if frac == 0 {
		return formatInt(whole)
	}
	s = formatInt(whole) + "." + formatInt(frac)
	return s
}

func formatInt(i int) string {
	if i < 0 {
		return "-" + formatUint(-i)
	}
	return formatUint(i)
}

func formatUint(i int) string {
	if i == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for i > 0 {
		buf = append(buf, byte('0'+i%10))
		i /= 10
	}
	// reverse
	for l, r := 0, len(buf)-1; l < r; l, r = l+1, r-1 {
		buf[l], buf[r] = buf[r], buf[l]
	}
	return string(buf)
}
