# pm2ui

A terminal UI for [PM2](https://pm2.keymetrics.io/) — manage your processes without leaving the terminal.

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

pm2ui connects to your local PM2 daemon and displays all running processes in an interactive table.

## Key Bindings

### Navigation

| Key        | Action                 |
|------------|------------------------|
| `j` / `↓` | Move down              |
| `k` / `↑` | Move up                |
| `/`        | Filter by name         |
| `Esc`      | Go back / clear filter |

### Process Actions

| Key     | Action                                      |
|---------|---------------------------------------------|
| `Enter` | View logs for selected process              |
| `Space` | Toggle multi-select (watch subset of logs)  |
| `u`     | Start stopped process                       |
| `r`     | Restart process                             |
| `s`     | Stop process                                |
| `d`     | Delete process                              |

### Sorting (Process Table)

| Key       | Action         |
|-----------|----------------|
| `Shift+N` | Sort by name   |
| `Shift+S` | Sort by status |
| `Shift+P` | Sort by PID    |
| `Shift+U` | Sort by uptime |

### Log Viewer

These keys work globally regardless of which panel has focus.

| Key | Action                                          |
|-----|-------------------------------------------------|
| `t` | Toggle stdout / stderr / both (single-process)  |
| `a` | Toggle autoscroll                               |
| `w` | Toggle word wrap                                |

### Log History

| Key      | Action              |
|----------|---------------------|
| `0`      | Tail (live from end)|
| `1`      | Head (from start)   |
| `2`      | Last 50 lines       |
| `3`      | Last 100 lines      |
| `4`      | Last 200 lines      |
| `5`      | Last 500 lines      |
| `6`      | Last 1000 lines     |

### Commands (`:`)

| Command        | Action                         |
|----------------|--------------------------------|
| `:restart all` | Restart all processes          |
| `:stop all`    | Stop all processes             |
| `:reload all`  | Graceful reload (cluster mode) |
| `:save`        | Persist process list to disk   |
| `:flush`       | Clear all log files            |
| `:q!`          | Quit                           |

### General

| Key   | Action                    |
|-------|---------------------------|
| `l`   | Focus logs panel (scroll) |
| `Esc` | Back to process list      |
| `?`   | Toggle help               |

## License

[MIT](LICENSE)
