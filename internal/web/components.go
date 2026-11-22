package web

type viewTab struct {
	Label   string
	Href    string
	Current bool
}

type viewNavigation struct {
	Label string
	Tabs  []viewTab
}

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
	Value     string
	Label     string
	Cells     []string
	Indicator string
	Disabled  bool
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
