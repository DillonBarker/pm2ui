package model

import (
	"testing"

	"github.com/DillonBarker/pm2ui/internal/pm2"
)

func proc(name, ns string) pm2.Process {
	p := pm2.Process{Name: name}
	p.PM2Env.Namespace = ns
	return p
}

func names(procs []pm2.Process) []string {
	out := make([]string, len(procs))
	for i, p := range procs {
		out[i] = p.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNamespaceFilter(t *testing.T) {
	pt := NewProcessTable()
	pt.Update([]pm2.Process{
		proc("api", "backend"),
		proc("web", "frontend"),
		proc("worker", "backend"),
	})

	pt.SetNamespace("backend")
	if got := names(pt.Processes()); !equal(got, []string{"api", "worker"}) {
		t.Errorf("namespace filter: got %v", got)
	}
	if pt.Namespace() != "backend" {
		t.Errorf("Namespace() = %q", pt.Namespace())
	}

	pt.SetNamespace("")
	if got := names(pt.Processes()); len(got) != 3 {
		t.Errorf("cleared namespace: got %v", got)
	}
}

func TestNamespaceFilter_CombinesWithNameFilter(t *testing.T) {
	pt := NewProcessTable()
	pt.Update([]pm2.Process{
		proc("api", "backend"),
		proc("api-gateway", "frontend"),
		proc("worker", "backend"),
	})

	pt.SetNamespace("backend")
	pt.SetFilter("api")
	if got := names(pt.Processes()); !equal(got, []string{"api"}) {
		t.Errorf("combined filters: got %v", got)
	}
}

func TestNamespaceFilter_SurvivesUpdate(t *testing.T) {
	pt := NewProcessTable()
	pt.SetNamespace("backend")
	pt.Update([]pm2.Process{
		proc("api", "backend"),
		proc("web", "frontend"),
	})
	if got := names(pt.Processes()); !equal(got, []string{"api"}) {
		t.Errorf("after update: got %v", got)
	}
}
