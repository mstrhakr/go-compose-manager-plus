package web

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mstrhakr/go-compose-manager-plus/internal/runtime"
)

type resourceDetailField struct {
	Label string
	Value string
	Href  string
}

type resourceDetailSection struct {
	Title  string
	Note   string
	Fields []resourceDetailField
}

type resourceDetailTab struct {
	ID       string
	Label    string
	Sections []resourceDetailSection
}

type resourceDetails struct {
	Key        string
	Kind       string
	Title      string
	Subtitle   string
	Notice     string
	Href       string
	CloseHref  string
	Tabs       []resourceDetailTab
	Activation *resourceDetailActivation
}

type resourceDetailActivation struct {
	Path      string
	CSRFToken string
	Disabled  bool
	Reason    string
}

func resourceDetailsURL(view string, query url.Values, key string) string {
	values := make(url.Values)
	for _, name := range []string{"q", "sort", "state", "group", "project"} {
		values[name] = query[name]
	}
	if key != "" {
		values.Set("details", key)
	}
	result := "/" + view
	if encoded := values.Encode(); encoded != "" {
		result += "?" + encoded
	}
	if key != "" {
		result += "#resource-details"
	}
	return result
}

func detailValue(value string) string {
	if value == "" {
		return "Not available"
	}
	return value
}

func containerDetails(container runtime.Container, query url.Values, message string) *resourceDetails {
	name := container.Name
	if name == "" {
		name = container.ID
	}
	created := "Not available"
	if container.Created > 0 {
		created = time.Unix(container.Created, 0).UTC().Format("Jan 2, 2006 15:04:05 UTC")
	}
	ports := resourceDetailSection{Title: "Ports", Note: "No port information reported in this snapshot."}
	for _, port := range container.Ports {
		value := fmt.Sprintf("%d/%s (not published)", port.PrivatePort, port.Type)
		if port.PublicPort > 0 {
			host := strconv.Itoa(int(port.PublicPort))
			if port.IP != "" {
				host = net.JoinHostPort(port.IP, host)
			}
			value = fmt.Sprintf("%s -> %d/%s", host, port.PrivatePort, port.Type)
		}
		ports.Fields = append(ports.Fields, resourceDetailField{Label: "Port mapping", Value: value})
	}
	if len(ports.Fields) > 0 {
		ports.Note = "Snapshot information only; this does not change published ports."
	}
	return &resourceDetails{
		Key: container.ID, Kind: "Container", Title: name, Subtitle: detailValue(container.Image),
		Notice: message, Href: resourceDetailsURL("containers", query, container.ID), CloseHref: resourceDetailsURL("containers", query, ""),
		Tabs: []resourceDetailTab{
			{ID: "overview", Label: "Overview", Sections: []resourceDetailSection{
				{Title: "Container", Fields: []resourceDetailField{
					{Label: "State", Value: detailValue(container.State)},
					{Label: "Status", Value: detailValue(container.Status)},
					{Label: "Created", Value: created},
					{Label: "Observability", Value: "Open Observability", Href: "/containers/observability?container=" + url.QueryEscape(container.ID)},
				}},
				{Title: "Compose association", Note: "Compose labels describe an association, not proof of exclusive ownership.", Fields: []resourceDetailField{
					{Label: "Project", Value: detailValue(container.Project)},
					{Label: "Service", Value: detailValue(container.Service)},
				}},
			}},
			{ID: "configuration", Label: "Configuration", Sections: []resourceDetailSection{
				{Title: "Identity", Note: "Read-only Docker snapshot. Environment variables and arbitrary labels are not displayed.", Fields: []resourceDetailField{
					{Label: "Container ID", Value: container.ID},
					{Label: "Name", Value: name},
					{Label: "Image", Value: detailValue(container.Image)},
				}},
				ports,
			}},
		},
	}
}

func stackDetails(project projectView, query url.Values, message string) *resourceDetails {
	runtimeName := detailValue(project.RuntimeName)
	containers := resourceDetailSection{Title: "Containers", Note: "Runtime association is unavailable in this snapshot."}
	if project.RuntimeName != "" {
		containers.Note = "No matching containers in this snapshot."
		for _, container := range project.Containers {
			name := container.Name
			if name == "" {
				name = container.ID
			}
			containers.Fields = append(containers.Fields, resourceDetailField{
				Label: detailValue(container.Service), Value: name + " - " + container.State,
				Href: resourceDetailsURL("containers", nil, container.ID),
			})
		}
		if len(containers.Fields) > 0 {
			containers.Note = "Projects sharing a Compose name show the same containers; this does not establish ownership."
		}
	}
	notice := message
	if project.RuntimeError != "" {
		notice += " " + project.RuntimeError
	}
	if project.ResolutionError != "" {
		notice += " " + project.ResolutionError
	}
	return &resourceDetails{
		Key: project.Path, Kind: "Stack", Title: project.DisplayName, Subtitle: runtimeName, Notice: notice,
		Href: resourceDetailsURL("projects", query, project.Path), CloseHref: resourceDetailsURL("projects", query, ""),
		Tabs: []resourceDetailTab{
			{ID: "overview", Label: "Overview", Sections: []resourceDetailSection{
				{Title: "Stack", Fields: []resourceDetailField{
					{Label: "Discovery", Value: project.DiscoveryStatus()},
					{Label: "Activation", Value: project.ActivationStatus()},
					{Label: "Compose project", Value: runtimeName},
				}, Note: "Activation grants permission only. Opening details does not run Compose or change project files."},
				containers,
			}},
			{ID: "configuration", Label: "Configuration", Sections: []resourceDetailSection{
				{Title: "Project location", Note: "Application-visible paths. Availability reflects the last successful scan.", Fields: []resourceDetailField{
					{Label: "Folder", Value: project.Path},
					{Label: "Discovery root", Value: project.Root},
				}},
				{Title: "Compose inputs", Note: "Native Compose input selection is retained. Resolved environment values and raw configuration are not displayed.", Fields: []resourceDetailField{
					{Label: "Runtime name", Value: runtimeName},
				}},
			}},
		},
	}
}

func (server *authServer) renderResourceDetails(w http.ResponseWriter, r *http.Request, details *resourceDetails) {
	var result bytes.Buffer
	if err := server.pages.ExecuteTemplate(&result, "resource-details", details); err != nil {
		server.logger.ErrorContext(r.Context(), "render resource details", "error", err)
		http.Error(w, "Unable to render resource details", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = result.WriteTo(w)
}
