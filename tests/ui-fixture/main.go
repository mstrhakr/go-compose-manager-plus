// Command ui-fixture renders component examples for browser tests only.
package main

import (
	"html/template"
	"log"
	"os"
)

type metric struct {
	ID, Title, Status, Value, Detail string
}
type chart struct {
	ID, Title, Status, EmptyTitle, EmptyDetail, Series string
}
type row struct {
	Value, Label string
	Cells        []string
	Disabled     bool
	Indicator    string
}
type list struct {
	ID, Title, Status, Empty string
	Selectable               bool
	Customize                bool
	Columns                  []string
	Rows                     []row
}

func main() {
	t := template.Must(template.ParseFiles("internal/web/templates/partials/components.html", "tests/ui-fixture/page.html"))
	data := struct {
		Metric            metric
		Chart, Empty, Bad chart
		List, Other       list
	}{
		Metric: metric{"metric-title", "CPU usage", "Sample", "42%", "Test fixture only"},
		Chart: chart{"chart-title", "Activity", "Sample", "No data", "No observations.",
			`{"min":0,"max":100,"series":[{"label":"CPU","color":"blue","points":[{"x":1,"y":0},{"x":2,"y":50},{"x":3,"y":null},{"x":4,"y":100}]},{"label":"Memory","color":"amber","points":[{"x":1,"y":25},{"x":4,"y":75}]}]}`},
		Empty: chart{"empty-title", "Empty chart", "Unavailable", "No data", "No observations.", ""},
		Bad:   chart{"bad-title", "Invalid chart", "Error", "No data", "No observations.", `{"min":0,"max":100,"series":[{"label":"Bad","color":"blue","points":[{"x":1,"y":101}]}]}`},
		List: list{"projects-title", "Projects", "Fixture", "", true, true, []string{"Name", "Status"}, []row{
			{"alpha", "Alpha", []string{"Alpha", "Available"}, false, ""},
			{"beta", "Beta", []string{"Beta", "Unavailable"}, false, ""},
			{"locked", "Locked", []string{"Locked", "Unavailable"}, true, ""},
		}},
		Other: list{"other-title", "Other resources", "Fixture", "", true, true, []string{"Name", "Status"}, []row{
			{"other", "Other", []string{"Other", "Available"}, false, ""},
		}},
	}
	if err := t.ExecuteTemplate(os.Stdout, "fixture", data); err != nil {
		log.Fatal(err)
	}
}
