package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestComponentTemplatesEscapeData(t *testing.T) {
	pages := template.Must(template.ParseFS(content, "templates/partials/*.html"))
	tests := []struct {
		name string
		data any
		want string
	}{
		{"metric-panel", metricPanel{ID: "metric", Title: "<metric>", Value: "<value>"}, "&lt;metric&gt;"},
		{"chart-panel", chartPanel{ID: "chart", Title: "<chart>", Series: `{"label":"<script>"}`}, "&lt;script&gt;"},
		{"view-navigation", viewNavigation{Label: "Views", Tabs: []viewTab{{Label: "<view>", Href: "/projects", Current: true}, {Label: "Future"}}}, "&lt;view&gt;"},
		{"list-panel", listPanel{ID: "list", Title: "Projects", Selectable: true, Columns: []string{"Name"}, Rows: []listRow{{Value: `"><script>`, Label: "<label>", Cells: []string{"<cell>"}}}}, "&lt;cell&gt;"},
		{"resource-details", resourceDetails{Key: `"><script>`, Title: "<resource>", Tabs: []resourceDetailTab{
			{ID: "overview", Label: "<tab>", Sections: []resourceDetailSection{{Title: "<section>", Fields: []resourceDetailField{
				{Label: "<field>", Value: "<value>", Href: "javascript:alert(1)"},
			}}}},
		}}, "&lt;resource&gt;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := pages.ExecuteTemplate(&output, tt.name, tt.data); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), tt.want) || strings.Contains(output.String(), "<script>") {
				t.Fatalf("unescaped component output: %s", output.String())
			}
		})
	}
}
