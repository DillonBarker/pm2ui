package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/DillonBarker/pm2ui/internal/pm2"
)

// newDescribeView renders full details for a process, k9s `describe` style.
// The view's text is refreshed on every watcher poll while open.
func newDescribeView(p pm2.Process) *tview.TextView {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetBackgroundColor(tcell.ColorDefault)
	tv.SetTextColor(tcell.ColorDefault)
	tv.SetBorderPadding(1, 1, 2, 2)
	tv.SetBorder(true)
	tv.SetBorderColor(tcell.ColorAqua)
	tv.SetTitle(fmt.Sprintf(" Describe — %s ", p.Name))
	tv.SetTitleColor(tcell.ColorAqua)
	tv.SetText(describeText(p))
	return tv
}

// describeText builds the describe page body for a process.
func describeText(p pm2.Process) string {
	env := p.PM2Env

	created := "-"
	if env.CreatedAt > 0 {
		created = time.UnixMilli(env.CreatedAt).Format("2006-01-02 15:04:05")
	}

	restarts := fmt.Sprintf("%d", env.RestartTime)
	if env.UnstableRestarts > 0 {
		restarts += fmt.Sprintf(" ([red]%d unstable[-])", env.UnstableRestarts)
	}

	autorestart := "off"
	if env.Autorestart {
		autorestart = "on"
	}

	esc := tview.Escape // user-controlled strings must not read as color tags
	rows := []struct {
		label string
		value string
	}{
		{"Name", esc(p.Name)},
		{"Namespace", orDash(esc(env.Namespace))},
		{"Status", fmt.Sprintf("[%s]%s[-]", statusToColor(env.Status), env.Status)},
		{"PID", fmt.Sprintf("%d", p.PID)},
		{"PM2 ID", fmt.Sprintf("%d", p.PM2ID)},
		{"Exec Mode", orDash(env.ExecMode)},
		{"Interpreter", orDash(esc(env.ExecInterpreter))},
		{"Node Version", orDash(esc(env.NodeVersion))},
		{"App Version", orDash(esc(env.Version))},
		{"Script", orDash(esc(env.PMExecPath))},
		{"CWD", orDash(esc(env.PMCwd))},
		{"Out Log", orDash(esc(env.PMOutLogPath))},
		{"Err Log", orDash(esc(env.PMErrLogPath))},
		{"Uptime", p.FormatUptime()},
		{"Created", created},
		{"Restarts", restarts},
		{"Autorestart", autorestart},
		{"CPU", fmt.Sprintf("%.1f%%", p.Monit.CPU)},
		{"Memory", p.FormatMemory()},
	}

	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "[yellow]%-14s[-] %s\n", r.label, r.value)
	}
	b.WriteString("\n[gray]Live — refreshes with each poll. Press Esc or i to close[-]")
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
