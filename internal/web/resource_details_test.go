package web

import (
	"bytes"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mstrhakr/go-compose-manager-plus/internal/discovery"
	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

func TestResourceDetailsPreserveFiltersAndFormatSnapshotMetadata(t *testing.T) {
	query := url.Values{"q": {"media & photos"}, "state": {"running", "exited"}, "sort": {"name-desc"}, "details": {"old"}, "password": {"excluded"}}
	container := runtime.Container{
		ID: strings.Repeat("a", 64), Image: "example/app:1", State: "running",
		Ports: []runtime.ContainerPort{
			{IP: "::", PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
			{PrivatePort: 443, Type: "tcp"},
		},
	}
	details := containerDetails(container, query, "Snapshot taken.")
	pages := template.Must(template.ParseFS(content, "templates/partials/*.html"))
	var output bytes.Buffer
	if err := pages.ExecuteTemplate(&output, "resource-details", details); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[::]:8080 -&gt; 80/tcp", "443/tcp (not published)", "Not available", container.ID} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q in snapshot details: %s", want, output.String())
		}
	}
	for _, href := range []string{details.Href, details.CloseHref} {
		link, err := url.Parse(href)
		if err != nil {
			t.Fatal(err)
		}
		values := link.Query()
		if values.Get("q") != "media & photos" || len(values["state"]) != 2 || values.Get("sort") != "name-desc" || values.Has("password") {
			t.Fatalf("filters not preserved safely: %s", href)
		}
	}
	if strings.Contains(details.CloseHref, "details=") || query.Get("details") != "old" {
		t.Fatalf("close link retained selection or mutated the caller's query: %s, %v", details.CloseHref, query)
	}
}

func TestSyntheticResourceDetailsPreview(t *testing.T) {
	pages := template.Must(template.ParseFS(content, "templates/*.html", "templates/partials/*.html"))
	server := authServer{pages: pages}
	containers := []runtime.Container{
		{ID: strings.Repeat("a", 64), Name: "media-services-web-1", Project: "media-services", Service: "web",
			State: "running", Status: "Up 2 hours (healthy)", Image: "registry.example/media/web:2026.11", Created: 1795500000,
			Ports: []runtime.ContainerPort{
				{IP: "0.0.0.0", PrivatePort: 8080, PublicPort: 18080, Type: "tcp"},
				{IP: "::", PrivatePort: 8080, PublicPort: 18080, Type: "tcp"},
				{PrivatePort: 8443, Type: "tcp"},
			}},
		{ID: strings.Repeat("b", 64), Name: "media-services-database-1", Project: "media-services", Service: "database",
			State: "exited", Status: "Exited (0) 4 minutes ago", Image: "postgres:17", Created: 1795500000},
	}
	project := projectView{
		Project:     discovery.Project{Path: "/projects/media-services", Root: "/projects", Active: true, Available: true},
		DisplayName: "media-services", RuntimeName: "media-services", Containers: containers,
		DetailsHref: resourceDetailsURL("projects", nil, "/projects/media-services"),
	}
	notice := "Synthetic preview only. No Docker connection or production data."
	stack := stackDetails(project, nil, notice)
	first, second := containerDetails(containers[0], nil, notice), containerDetails(containers[1], nil, notice)
	long := containers[0]
	long.Name = strings.Repeat("very-long-container-name-", 8)
	long.Image = strings.Repeat("registry.example/deeply-nested/image/", 6) + ":latest"
	for i := 0; i < 16; i++ {
		long.Ports = append(long.Ports, runtime.ContainerPort{PrivatePort: uint16(9000 + i), Type: "tcp"})
	}
	list := listPanel{ID: "containers-title", Title: "Containers", Status: "Snapshot", Customize: true,
		Columns: []string{"Name", "State", "Compose project", "Service"}}
	for _, container := range containers {
		indicator := "stopped"
		if container.State == "running" {
			indicator = "running"
		}
		list.Rows = append(list.Rows, listRow{Cells: []string{container.Name, container.State, container.Project, container.Service},
			Indicator: indicator, DetailsHref: resourceDetailsURL("containers", nil, container.ID)})
	}
	directory := os.Getenv("CMP_UI_PREVIEW_DIR")
	save := func(name, templateName string, data any) {
		t.Helper()
		var output bytes.Buffer
		if err := server.pages.ExecuteTemplate(&output, templateName, data); err != nil {
			t.Fatal(err)
		}
		if directory != "" {
			if err := os.WriteFile(filepath.Join(directory, name+".html"), output.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, detail := range map[string]*resourceDetails{"stack": stack, "container-a": first, "container-b": second, "container-long": containerDetails(long, nil, notice)} {
		save(name, "resource-details", detail)
	}
	base := pageData{Title: "Resource details preview", Username: "Preview", Initial: "P", Notice: notice, RuntimeState: "Snapshot", RuntimeMessage: notice, Root: "/projects"}
	base.View = "projects"
	base.Filters, base.Projects, _ = filterInventory("projects", nil, []projectView{project}, nil)
	base.ProjectList = projectInventoryList(base.Projects, base.Filters.Total, base.RuntimeState)
	save("projects", "page", base)
	base.ResourceDetails = stack
	save("projects-selected", "page", base)
	base.View, base.Projects, base.Containers, base.ResourceDetails = "containers", nil, list, nil
	base.Filters, _, _ = filterInventory("containers", nil, nil, containers)
	save("containers", "page", base)
	base.ResourceDetails = first
	save("containers-selected", "page", base)
}
