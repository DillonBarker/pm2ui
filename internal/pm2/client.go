package pm2

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	cmdTimeout     = 5 * time.Second
	bulkCmdTimeout = 30 * time.Second // For operations that affect all processes
)

// Client wraps PM2 CLI commands.
type Client struct{}

// NewClient creates a new PM2 client.
func NewClient() *Client {
	return &Client{}
}

// List runs `pm2 jlist` and returns parsed processes.
func (c *Client) List() ([]Process, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "pm2", "jlist").Output()
	if err != nil {
		return nil, fmt.Errorf("pm2 jlist: %w", err)
	}

	var procs []Process
	if err := json.Unmarshal(out, &procs); err != nil {
		return nil, fmt.Errorf("parse pm2 jlist: %w", err)
	}
	return procs, nil
}

// Restart restarts a process by name with --update-env to pick up env changes.
func (c *Client) Restart(name string) error {
	return c.runArgs("restart", name, "--update-env")
}

// Stop stops a process by name.
func (c *Client) Stop(name string) error {
	return c.runArgs("stop", name)
}

// Start starts a process by name.
func (c *Client) Start(name string) error {
	return c.runArgs("start", name)
}

// Delete deletes a process by name.
func (c *Client) Delete(name string) error {
	return c.runArgs("delete", name)
}

func (c *Client) run(args ...string) error {
	return c.runWithTimeout(cmdTimeout, args...)
}

func (c *Client) runWithTimeout(timeout time.Duration, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "pm2", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pm2 %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

func (c *Client) runArgs(command, name string, extraArgs ...string) error {
	return c.run(append([]string{command, name}, extraArgs...)...)
}

// RestartAll restarts every process.
func (c *Client) RestartAll() error { return c.runWithTimeout(bulkCmdTimeout, "restart", "all") }

// StopAll stops every process.
func (c *Client) StopAll() error { return c.runWithTimeout(bulkCmdTimeout, "stop", "all") }

// ReloadAll gracefully reloads every process (cluster mode).
func (c *Client) ReloadAll() error { return c.runWithTimeout(bulkCmdTimeout, "reload", "all") }

// Save persists the process list to disk.
func (c *Client) Save() error { return c.run("save") }

// Flush clears all log files.
func (c *Client) Flush() error { return c.run("flush") }
