# pm2ui

A terminal UI for [PM2](https://pm2.keymetrics.io/) — manage your processes without leaving the terminal. Inspired by [k9s](https://k9scli.io/).

## Features

- **Live process table** — status, PID, CPU, memory, restart count, uptime; sortable by any column
- **Streaming logs** — all services merged with per-service colors, or drill into one; survives `pm2 flush` and log rotation
- **Log search** — case-insensitive regex with highlighted matches (`/`)
- **Multi-select** — Space-select services to filter the log stream and run bulk restart/stop/delete
- **Crash alerts** — flash notifications when a service errors or its restart count climbs
- **Log tools** — pause, timestamped marks, fullscreen, save to file, history depth presets
- **Describe view** — live per-process detail (script path, versions, log paths, stats)
- **Namespaces** — scope the table to a pm2 namespace (`:ns`)
- **Mouse support** and a `~/.config/pm2ui/config.yaml` for tuning

## Prerequisites

- [PM2](https://pm2.keymetrics.io/) installed and accessible in your `$PATH`

## Installation

### go install

```sh
go install github.com/DillonBarker/pm2ui@latest
```

### Download a binary

Pre-built binaries for Linux, macOS, and Windows are available on the [releases page](https://github.com/DillonBarker/pm2ui/releases).

Download the archive for your platform, extract it, and place the binary somewhere in your `$PATH`:

```sh
# macOS (Apple Silicon)
curl -LO https://github.com/DillonBarker/pm2ui/releases/latest/download/pm2ui_Darwin_arm64.tar.gz
tar -xzf pm2ui_Darwin_arm64.tar.gz
mv pm2ui /usr/local/bin/

# macOS (Intel)
curl -LO https://github.com/DillonBarker/pm2ui/releases/latest/download/pm2ui_Darwin_amd64.tar.gz
tar -xzf pm2ui_Darwin_amd64.tar.gz
mv pm2ui /usr/local/bin/

# Linux (amd64)
curl -LO https://github.com/DillonBarker/pm2ui/releases/latest/download/pm2ui_Linux_amd64.tar.gz
tar -xzf pm2ui_Linux_amd64.tar.gz
mv pm2ui /usr/local/bin/
```

### Build from source

```sh
git clone https://github.com/DillonBarker/pm2ui.git
cd pm2ui
go build -o pm2ui .
```

## Usage

```sh
pm2ui
```

pm2ui connects to your local PM2 daemon and displays all running processes in an interactive table. Press `?` inside the app for the full key reference; `pm2ui --version` prints the version.

## Key Bindings

### Navigation

| Key        | Action                 |
|------------|------------------------|
| `j` / `↓` | Move down              |
| `k` / `↑` | Move up                |
| `/`        | Filter by name         |
| `Esc`      | Go back / clear filter |

### Process Actions

| Key     | Action                                           |
|---------|--------------------------------------------------|
| `Enter` | View logs for selected process                   |
| `i`     | Describe process (live: paths, versions, stats)  |
| `Space` | Toggle multi-select (filters logs, bulk actions) |
| `u`     | Start stopped process(es)                        |
| `r`     | Restart process(es)                              |
| `s`     | Stop process(es)                                 |
| `d`     | Delete process(es)                               |

With a Space-selection active, `u`/`r`/`s`/`d` act on **all selected**
services (bulk restart of a whole group in one keypress, k9s-style).

### Sorting (Process Table)

| Key       | Action           |
|-----------|------------------|
| `Shift+N` | Sort by name     |
| `Shift+S` | Sort by status   |
| `Shift+P` | Sort by PID      |
| `Shift+C` | Sort by CPU      |
| `Shift+M` | Sort by memory   |
| `Shift+R` | Sort by restarts |
| `Shift+U` | Sort by uptime   |

### Log Viewer

These keys work globally regardless of which panel has focus (except `/` and
`S`, which act on logs only while the log panel is focused).

| Key | Action                                                   |
|-----|----------------------------------------------------------|
| `/` | Search logs — case-insensitive regex, matches highlighted|
| `m` | Insert a timestamped mark line (see what's new since)    |
| `c` | Clear panel (tailing, search and selection keep running) |
| `t` | Toggle stdout / stderr / both                            |
| `a` | Toggle autoscroll                                        |
| `w` | Toggle word wrap                                         |
| `p` | Pause/resume log tailing (buffers while paused)          |
| `T` | Toggle arrival timestamps                                |
| `f` | Fullscreen logs                                          |
| `S` | Save the log buffer to a file in the temp dir            |

Scrolling up (`↑`, `PgUp`, `k`, `g`) pauses tailing automatically; jump back
to the end (`G`, `End`) to resume. Logs survive `pm2 flush` and log rotation.

In merged (all/multi) mode, lines are interleaved in arrival order — pm2 log
files don't carry reliable timestamps, so use `T` to stamp lines as they
arrive.

### Log History

| Key      | Action              |
|----------|---------------------|
| `0`      | Tail (live from end)|
| `1`      | Head (first 200)    |
| `2`      | Last 50 lines       |
| `3`      | Last 100 lines      |
| `4`      | Last 200 lines      |
| `5`      | Last 500 lines      |
| `6`      | Last 1000 lines     |

Opening a single service shows its last 200 lines; the merged all-services
view starts with 50 lines per service. Both are configurable
(`defaultTailLines` / `allTailLines`), and `0`–`6` override either on the fly.

### Commands (`:`)

Command mode opens from any pane; `Tab` completes commands and namespace
names.

| Command        | Action                                    |
|----------------|-------------------------------------------|
| `:ns <name>`   | Scope table to a pm2 namespace (`:ns` clears) |
| `:restart all` | Restart all processes                     |
| `:stop all`    | Stop all processes                        |
| `:reload all`  | Graceful reload (cluster mode)            |
| `:save`        | Persist process list to disk              |
| `:flush`       | Clear all log files                       |
| `:q` / `:q!`   | Quit                                      |

### General

| Key   | Action                    |
|-------|---------------------------|
| `l`   | Focus logs panel (scroll) |
| `Esc` | Back to process list      |
| `?`   | Toggle help               |

The focused pane has an aqua border. Mouse is supported: click to focus a
pane or select a row; wheel-up pauses tailing like keyboard scrolling.
Terminal text selection usually needs a modifier key while mouse mode is on
(Option/Shift depending on terminal) — set `mouseEnabled: false` to opt out.

## Configuration

pm2ui reads `~/.config/pm2ui/config.yaml` (or `$XDG_CONFIG_HOME/pm2ui/config.yaml`)
if present. All keys are optional:

```yaml
refreshInterval: 2s    # how often the process list polls pm2
defaultTailLines: 200  # history shown when opening single-service logs
allTailLines: 50       # initial history per service in the merged view
maxLogLines: 5000      # log buffer cap (fair-shared across services)
logBatchInterval: 50ms # how often new log lines are flushed to screen
splitRatio: 2          # width of the logs pane relative to the table (1-5)
mouseEnabled: true     # click to focus/select, wheel scroll
```

Invalid config falls back to defaults and shows a warning on start.

## License

[MIT](LICENSE)
