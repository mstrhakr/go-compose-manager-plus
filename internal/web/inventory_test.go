package web

import (
	"net/url"
	"testing"

	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestInventoryFilteringAndSorting(t *testing.T) {
	projects := []projectView{
		{Project: discovery.Project{Path: "/root/beta", Available: true}, DisplayName: "beta"},
		{Project: discovery.Project{Path: "/root/alpha", Available: true, Active: true}, DisplayName: "alpha"},
		{Project: discovery.Project{Path: "/root/old", Available: false}, DisplayName: "old"},
	}

	filters, got, _ := filterInventory("projects", url.Values{"state": {"Available"}, "sort": {"name-desc"}}, projects, nil)
	if filters.Total != 3 || filters.Shown != 2 || got[0].DisplayName != "beta" || got[1].DisplayName != "alpha" {
		t.Fatalf("projects: %#v %#v", filters, got)
	}

	containers := []runtime.Container{
		{Name: "web", Project: "alpha", State: "running"},
		{Name: "db", Project: "alpha", State: "exited"},
		{Name: "standalone", State: "running"},
	}
	filters, _, rows := filterInventory("containers", url.Values{"state": {"running", "exited"}, "project": {"alpha"}, "sort": {"name-desc"}}, nil, containers)
	if filters.Total != 3 || filters.Shown != 2 || rows[0].Name != "web" || rows[1].Name != "db" {
		t.Fatalf("containers: %#v %#v", filters, rows)
	}
	filters, _, rows = filterInventory("containers", url.Values{"state": {"missing"}}, nil, containers)
	if filters.Shown != 0 || len(rows) != 0 {
		t.Fatalf("missing selection: %#v %#v", filters, rows)
	}
	found := false
	for _, option := range filters.States {
		if option.Value == "missing" && option.Checked && option.Count == 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("stale filter selection not retained: %#v", filters.States)
	}
}

func TestProjectStatusLabels(t *testing.T) {
	tests := []struct {
		project    projectView
		discovery  string
		activation string
	}{
		{projectView{Project: discovery.Project{Available: true, Active: true}}, "Available", "Active"},
		{projectView{Project: discovery.Project{Available: true, ResolutionError: "invalid"}}, "Inputs need attention", "Inactive"},
		{projectView{Project: discovery.Project{ResolutionError: "invalid"}}, "Unavailable", "Inactive"},
	}
	for _, tt := range tests {
		if got := tt.project.DiscoveryStatus(); got != tt.discovery {
			t.Errorf("discovery status = %q, want %q", got, tt.discovery)
		}
		if got := tt.project.ActivationStatus(); got != tt.activation {
			t.Errorf("activation status = %q, want %q", got, tt.activation)
		}
	}
}
