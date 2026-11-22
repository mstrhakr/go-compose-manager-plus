package web

import (
	"net/url"
	"sort"
	"strings"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

type filterOption struct {
	Value   string
	Label   string
	Count   int
	Checked bool
}

type inventoryFilters struct {
	Path     string
	Search   string
	Sort     string
	States   []filterOption
	Groups   []filterOption
	Projects []filterOption
	Total    int
	Shown    int
}

func selected(values url.Values, key string) map[string]bool {
	result := make(map[string]bool)
	for _, value := range values[key] {
		result[value] = true
	}
	return result
}

func matchesSelection(selection map[string]bool, value string) bool {
	return len(selection) == 0 || selection[value]
}

func filterOptions(counts map[string]int, selection map[string]bool) []filterOption {
	for value := range selection {
		if _, ok := counts[value]; !ok {
			counts[value] = 0
		}
	}
	options := make([]filterOption, 0, len(counts))
	for value, count := range counts {
		options = append(options, filterOption{Value: value, Label: value, Count: count, Checked: selection[value]})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Label < options[j].Label })
	return options
}

func filterInventory(view string, query url.Values, projects []projectView, containers []runtime.Container) (inventoryFilters, []projectView, []runtime.Container) {
	filters := inventoryFilters{Path: "/" + view, Search: strings.TrimSpace(query.Get("q")), Sort: query.Get("sort")}
	if filters.Sort != "name-desc" {
		filters.Sort = "name"
	}
	search := strings.ToLower(filters.Search)
	states, groups, projectNames := selected(query, "state"), selected(query, "group"), selected(query, "project")
	stateCounts, groupCounts, projectCounts := make(map[string]int), make(map[string]int), make(map[string]int)
	if view == "projects" {
		filters.Total = len(projects)
		filtered := make([]projectView, 0, len(projects))
		for _, project := range projects {
			state := project.DiscoveryStatus()
			group := project.ActivationStatus()
			stateCounts[state]++
			groupCounts[group]++
			if strings.Contains(strings.ToLower(project.Path+" "+project.RuntimeName), search) && matchesSelection(states, state) && matchesSelection(groups, group) {
				filtered = append(filtered, project)
			}
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			left := strings.ToLower(filtered[i].DisplayName) + "\x00" + filtered[i].Path
			right := strings.ToLower(filtered[j].DisplayName) + "\x00" + filtered[j].Path
			if filters.Sort == "name-desc" {
				return left > right
			}
			return left < right
		})
		filters.Shown = len(filtered)
		projects = filtered
	} else {
		filters.Total = len(containers)
		filtered := make([]runtime.Container, 0, len(containers))
		for _, container := range containers {
			group := "Compose"
			if container.Project == "" {
				group = "No Compose project label"
			}
			stateCounts[container.State]++
			groupCounts[group]++
			if container.Project != "" {
				projectCounts[container.Project]++
			}
			if strings.Contains(strings.ToLower(container.Name+" "+container.ID+" "+container.Project+" "+container.Service), search) && matchesSelection(states, container.State) && matchesSelection(groups, group) && matchesSelection(projectNames, container.Project) {
				filtered = append(filtered, container)
			}
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			left := strings.ToLower(filtered[i].Name)
			right := strings.ToLower(filtered[j].Name)
			if left == "" {
				left = strings.ToLower(filtered[i].ID)
			}
			if right == "" {
				right = strings.ToLower(filtered[j].ID)
			}
			if filters.Sort == "name-desc" {
				return left > right
			}
			return left < right
		})
		filters.Shown = len(filtered)
		containers = filtered
	}
	filters.States = filterOptions(stateCounts, states)
	filters.Groups = filterOptions(groupCounts, groups)
	if view == "containers" {
		filters.Projects = filterOptions(projectCounts, projectNames)
	}
	return filters, projects, containers
}
