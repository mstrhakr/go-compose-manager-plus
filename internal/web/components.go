package web

type metricPanel struct {
	ID     string
	Title  string
	Status string
	Value  string
	Detail string
}

type chartPanel struct {
	ID          string
	Title       string
	Status      string
	EmptyTitle  string
	EmptyDetail string
	Series      string
}

type listRow struct {
	Value    string
	Label    string
	Cells    []string
	Disabled bool
}

type listPanel struct {
	ID         string
	Title      string
	Status     string
	Empty      string
	Selectable bool
	Columns    []string
	Rows       []listRow
}
