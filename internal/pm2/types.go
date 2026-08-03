package pm2

import (
	"strconv"
	"strings"
	"time"
)

// Process represents a PM2 process from `pm2 jlist` output.
type Process struct {
	PID    int    `json:"pid"`
	Name   string `json:"name"`
	PM2ID  int    `json:"pm_id"`
	Monit  Monit  `json:"monit"`
	PM2Env PM2Env `json:"pm2_env"`
}

// Monit holds resource usage metrics.
type Monit struct {
	Memory int64   `json:"memory"`
	CPU    float64 `json:"cpu"`
}

// PM2Env holds the PM2 environment configuration for a process.
type PM2Env struct {
	Status           string `json:"status"`
	PMUptime         int64  `json:"pm_uptime"`
	RestartTime      int    `json:"restart_time"`
	UnstableRestarts int    `json:"unstable_restarts"`
	PMOutLogPath     string `json:"pm_out_log_path"`
	PMErrLogPath     string `json:"pm_err_log_path"`
	ExecMode         string `json:"exec_mode"`
	NodeVersion      string `json:"node_version"`
	Namespace        string `json:"namespace"`
	PMExecPath       string `json:"pm_exec_path"`
	PMCwd            string `json:"pm_cwd"`
	ExecInterpreter  string `json:"exec_interpreter"`
	CreatedAt        int64  `json:"created_at"`
	Version          string `json:"version"`
	Autorestart      bool   `json:"autorestart"`
}

// StatusOnline is the PM2 status for a running process.
const (
	StatusOnline    = "online"
	StatusStopped   = "stopped"
	StatusErrored   = "errored"
	StatusStopping  = "stopping"
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
	mem := float64(p.Monit.Memory)
	switch {
	case p.Monit.Memory >= 1<<30:
		return trimDecimal(mem/(1<<30)) + " GB"
	case p.Monit.Memory >= 1<<20:
		return trimDecimal(mem/(1<<20)) + " MB"
	case p.Monit.Memory >= 1<<10:
		return trimDecimal(mem/(1<<10)) + " KB"
	default:
		return strconv.FormatInt(p.Monit.Memory, 10) + " B"
	}
}

// trimDecimal formats with one decimal place, dropping a trailing .0.
func trimDecimal(f float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(f, 'f', 1, 64), ".0")
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
		return strconv.Itoa(days) + "d" + strconv.Itoa(hours) + "h"
	case hours > 0:
		return strconv.Itoa(hours) + "h" + strconv.Itoa(mins) + "m"
	default:
		secs := int(d.Seconds()) % 60
		return strconv.Itoa(mins) + "m" + strconv.Itoa(secs) + "s"
	}
}
