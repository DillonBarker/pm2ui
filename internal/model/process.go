package model

import (
	"sort"
	"strings"
	"sync"

	"github.com/DillonBarker/pm2ui/internal/pm2"
)

// SortColumn identifies a column for sorting.
type SortColumn int

const (
	SortByName SortColumn = iota
	SortByStatus
	SortByPID
	SortByUptime
)

// ProcessTableListener is called when the filtered/sorted view changes.
type ProcessTableListener func([]pm2.Process)

// ProcessTable holds the process list state with filtering and sorting.
type ProcessTable struct {
	mu        sync.RWMutex
	raw       []pm2.Process
	filtered  []pm2.Process
	filter    string
	sortCol   SortColumn
	sortAsc   bool
	listeners []ProcessTableListener
}

// NewProcessTable creates a new ProcessTable.
func NewProcessTable() *ProcessTable {
	return &ProcessTable{
		sortCol: SortByName,
		sortAsc: true,
	}
}

// OnChange registers a listener for view changes.
func (pt *ProcessTable) OnChange(fn ProcessTableListener) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.listeners = append(pt.listeners, fn)
}

// Update replaces the raw process list and recomputes the view.
func (pt *ProcessTable) Update(procs []pm2.Process) {
	pt.mu.Lock()
	pt.raw = procs
	pt.recompute()
	filtered := make([]pm2.Process, len(pt.filtered))
	copy(filtered, pt.filtered)
	listeners := make([]ProcessTableListener, len(pt.listeners))
	copy(listeners, pt.listeners)
	pt.mu.Unlock()

	for _, fn := range listeners {
		fn(filtered)
	}
}

// SetFilter sets the name filter and recomputes. It does NOT notify listeners —
// callers on the event loop must update the UI directly after this call.
func (pt *ProcessTable) SetFilter(f string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.filter = f
	pt.recompute()
}

// SetSort sets the sort column; toggles direction if same column.
func (pt *ProcessTable) SetSort(col SortColumn) {
	pt.mu.Lock()
	if pt.sortCol == col {
		pt.sortAsc = !pt.sortAsc
	} else {
		pt.sortCol = col
		pt.sortAsc = true
	}
	pt.recompute()
	filtered := make([]pm2.Process, len(pt.filtered))
	copy(filtered, pt.filtered)
	listeners := make([]ProcessTableListener, len(pt.listeners))
	copy(listeners, pt.listeners)
	pt.mu.Unlock()

	for _, fn := range listeners {
		fn(filtered)
	}
}

// Filter returns the current filter string.
func (pt *ProcessTable) Filter() string {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.filter
}

// SortInfo returns the current sort column and direction.
func (pt *ProcessTable) SortInfo() (SortColumn, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.sortCol, pt.sortAsc
}

// Processes returns the current filtered/sorted view.
func (pt *ProcessTable) Processes() []pm2.Process {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	result := make([]pm2.Process, len(pt.filtered))
	copy(result, pt.filtered)
	return result
}

// Raw returns the unfiltered process list.
func (pt *ProcessTable) Raw() []pm2.Process {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	result := make([]pm2.Process, len(pt.raw))
	copy(result, pt.raw)
	return result
}

// TotalCount returns the number of unfiltered processes without copying the slice.
func (pt *ProcessTable) TotalCount() int {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return len(pt.raw)
}

// recompute applies filter and sort. Must be called with mu held.
func (pt *ProcessTable) recompute() {
	// Filter
	if pt.filter == "" {
		pt.filtered = make([]pm2.Process, len(pt.raw))
		copy(pt.filtered, pt.raw)
	} else {
		pt.filtered = pt.filtered[:0]
		lower := strings.ToLower(pt.filter)
		for _, p := range pt.raw {
			if strings.Contains(strings.ToLower(p.Name), lower) {
				pt.filtered = append(pt.filtered, p)
			}
		}
	}

	// Sort
	col := pt.sortCol
	asc := pt.sortAsc
	sort.SliceStable(pt.filtered, func(i, j int) bool {
		a, b := pt.filtered[i], pt.filtered[j]
		var less bool
		switch col {
		case SortByName:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case SortByStatus:
			less = a.PM2Env.Status < b.PM2Env.Status
		case SortByPID:
			less = a.PID < b.PID
		case SortByUptime:
			less = a.Uptime() < b.Uptime()
		}
		if !asc {
			return !less
		}
		return less
	})
}
