package ui

import (
	"fmt"
	"strings"

	"github.com/DillonBarker/pm2ui/internal/pm2"
)

// titleNameBudget caps the characters spent on process names so the title
// still fits a terminal tab. Names beyond the budget collapse into "+N".
const titleNameBudget = 24

// WindowTitle builds the terminal window/tab title from the full (unfiltered)
// process list: an online count plus the names of everything that isn't
// online, so the tab bar alone says what's up and what's down.
//
//	pm2ui ●8              all online
//	pm2ui ●6 ✖api,worker  two down, named
//	pm2ui ●3 ✖api,web,+3  more down than fit
//	pm2ui                 pm2 has no processes
func WindowTitle(procs []pm2.Process) string {
	const base = "pm2ui"
	if len(procs) == 0 {
		return base
	}

	var online int
	var errored bool
	var down []string
	for _, p := range procs {
		if p.PM2Env.Status == pm2.StatusOnline {
			online++
			continue
		}
		if p.PM2Env.Status == pm2.StatusErrored {
			errored = true
		}
		down = append(down, p.Name)
	}

	title := fmt.Sprintf("%s ●%d", base, online)
	if len(down) == 0 {
		return title
	}

	// Errored is the louder state: use its marker whenever anything errored,
	// otherwise the group is stopped/stopping/launching.
	marker := "⏸"
	if errored {
		marker = "✖"
	}
	return fmt.Sprintf("%s %s%s", title, marker, joinWithinBudget(down, titleNameBudget))
}

// joinWithinBudget joins names with "," while the result stays within budget
// runes, replacing the ones that don't fit with a "+N" tally. At least one
// name is always kept so the title names a culprit even when names are long.
func joinWithinBudget(names []string, budget int) string {
	var kept []string
	used := 0
	for i, name := range names {
		cost := len([]rune(name))
		if i > 0 {
			cost++ // separator
		}
		remaining := len(names) - i
		if i > 0 && used+cost+overflowCost(remaining) > budget {
			return strings.Join(kept, ",") + fmt.Sprintf(",+%d", remaining)
		}
		kept = append(kept, name)
		used += cost
	}
	return strings.Join(kept, ",")
}

// overflowCost is the width of the ",+N" suffix needed if we stop here.
func overflowCost(remaining int) int {
	return len(fmt.Sprintf(",+%d", remaining))
}
