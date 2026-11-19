package components

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	xwidget "fyne.io/x/fyne/widget"
	"go.qbee.io/client"
	"go.qbee.io/connect/internal/testutil"
)

func group(id, title string, children ...client.GroupTreeNode) client.GroupTreeNode {
	return client.GroupTreeNode{NodeID: id, Title: title, Type: client.NodeTypeGroup, Nodes: children}
}

func device(id, title string) client.GroupTreeNode {
	return client.GroupTreeNode{NodeID: id, Title: title, Type: client.NodeTypeDevice}
}

func testGroupTree() *client.GroupTree {
	return &client.GroupTree{Tree: group("root", "root",
		group("eu", "Europe",
			group("no", "Norway",
				group("osl", "Oslo"),
				group("brg", "Bergen"),
				device("d1", "gateway-1"),
			),
			group("de", "Germany"),
		),
		group("us", "Americas"),
		device("d2", "gateway-2"),
	)}
}

func TestGetGroupsBreadcrumb(t *testing.T) {
	got := getGroupsBreadcrumb(*testGroupTree())
	want := map[string]string{
		"root":                            "root",
		"root > Europe":                   "eu",
		"root > Europe > Norway":          "no",
		"root > Europe > Norway > Oslo":   "osl",
		"root > Europe > Norway > Bergen": "brg",
		"root > Europe > Germany":         "de",
		"root > Americas":                 "us",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("breadcrumbs mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestGetGroupsBreadcrumb_Empty(t *testing.T) {
	if got := getGroupsBreadcrumb(client.GroupTree{}); len(got) != 0 {
		t.Fatalf("expected no breadcrumbs for an empty tree, got %v", got)
	}
}

func TestDeviceSearchEntry_SubmitSetsTitleQuery(t *testing.T) {
	d := newFakeDelegate(t)
	entry := newDeviceSearchEntry(d)
	w := test.NewTempWindow(t, entry)

	if entry.PlaceHolder != "Search by device name..." {
		t.Errorf("placeholder = %q", entry.PlaceHolder)
	}

	w.Canvas().Focus(entry)
	test.Type(entry, "gateway-01")
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if q := d.lastQuery(t); q.Title != "gateway-01" {
		t.Fatalf("query = %+v, want title gateway-01", q)
	}
}

func TestSearchBar_DefaultsToDeviceNameSearch(t *testing.T) {
	d := newFakeDelegate(t)
	bar := NewSearchBar(d)
	test.NewTempWindow(t, bar)

	sel, ok := testutil.Find[*widget.Select](bar, nil)
	if !ok {
		t.Fatal("search type selector not found")
	}
	if sel.Selected != searchDeviceName {
		t.Fatalf("selected = %q, want %q", sel.Selected, searchDeviceName)
	}
	if _, ok := testutil.Find(bar, func(e *widget.Entry) bool { return e.PlaceHolder == "Search by device name..." }); !ok {
		t.Fatal("device name entry not shown by default")
	}
}

func TestSearchBar_RefreshButton(t *testing.T) {
	d := newFakeDelegate(t)
	bar := NewSearchBar(d)
	test.NewTempWindow(t, bar)

	test.Tap(findButton(t, bar, "Refresh"))

	if load, _ := d.refreshes(); load != 1 {
		t.Fatalf("RefreshUI called %d times, want 1", load)
	}
}

func TestSearchBar_SwitchingSearchType(t *testing.T) {
	tests := []struct {
		kind        string
		placeholder string
	}{
		{searchTag, "Search for tag.."},
		{searchGroup, "Search for group.."},
		{searchDeviceName, "Search by device name..."},
	}

	d := newFakeDelegate(t)
	d.groups = testGroupTree()
	bar := NewSearchBar(d)
	test.NewTempWindow(t, bar)
	sel, _ := testutil.Find[*widget.Select](bar, nil)

	for _, tt := range tests {
		sel.SetSelected(tt.kind)
		entries := testutil.FindAll(bar, func(e *widget.Entry) bool { return e.PlaceHolder != "" })
		// CompletionEntry embeds widget.Entry, so look at both concrete types.
		completions := testutil.FindAll(bar, func(e *xwidget.CompletionEntry) bool { return e.PlaceHolder != "" })

		var placeholders []string
		for _, e := range entries {
			placeholders = append(placeholders, e.PlaceHolder)
		}
		for _, e := range completions {
			placeholders = append(placeholders, e.PlaceHolder)
		}
		if len(placeholders) != 1 || placeholders[0] != tt.placeholder {
			t.Errorf("%s: visible inputs %v, want only %q", tt.kind, placeholders, tt.placeholder)
		}
	}
}

func newTagSearch(t *testing.T, tags ...string) (*fakeDelegate, *xwidget.CompletionEntry) {
	t.Helper()
	d := newFakeDelegate(t)
	d.tags = tags
	e := newTagsSearchComplete(d)
	w := test.NewTempWindow(t, e)
	w.Resize(fyne.NewSize(400, 300))
	w.Canvas().Focus(e)
	return d, e
}

func TestTagSearch_FiltersOptionsWhileTyping(t *testing.T) {
	_, e := newTagSearch(t, "prod", "lab", "preprod", "edge")

	test.Type(e, "pr")
	if want := []string{"preprod", "prod"}; !reflect.DeepEqual(e.Options, want) {
		t.Fatalf("options = %v, want %v", e.Options, want)
	}

	e.SetText("")
	if want := []string{"edge", "lab", "preprod", "prod"}; !reflect.DeepEqual(e.Options, want) {
		t.Fatalf("options for empty search = %v, want all tags sorted %v", e.Options, want)
	}
}

func TestTagSearch_SubmitExactMatch(t *testing.T) {
	d, e := newTagSearch(t, "prod", "preprod")

	test.Type(e, "prod")
	e.OnSubmitted("prod")

	if q := d.lastQuery(t); !reflect.DeepEqual(q.Tags, []string{"prod"}) {
		t.Fatalf("query tags = %v, want [prod]", q.Tags)
	}
	if e.Text != "prod" {
		t.Fatalf("entry text = %q", e.Text)
	}
}

func TestTagSearch_SubmitPartialPicksFirstOption(t *testing.T) {
	d, e := newTagSearch(t, "prod", "preprod")

	test.Type(e, "pre")
	e.OnSubmitted("pre")

	if q := d.lastQuery(t); !reflect.DeepEqual(q.Tags, []string{"preprod"}) {
		t.Fatalf("query tags = %v, want [preprod]", q.Tags)
	}
	if e.Text != "preprod" {
		t.Fatalf("entry text = %q, want completed value", e.Text)
	}
}

func TestTagSearch_SubmitIgnoredWithoutMatchOrInput(t *testing.T) {
	d, e := newTagSearch(t, "prod")

	e.OnSubmitted("prod") // no options computed yet
	test.Type(e, "zzz")   // no matching options
	e.OnSubmitted("zzz")
	e.SetText(" ")
	e.OnSubmitted(" ") // blank input

	if len(d.searchQueries) != 0 {
		t.Fatalf("expected no search queries, got %v", d.searchQueries)
	}
}

func TestGroupSearch_FilterAndSubmit(t *testing.T) {
	d := newFakeDelegate(t)
	d.groups = testGroupTree()
	e := newGroupSearchComplete(d)
	w := test.NewTempWindow(t, e)
	w.Resize(fyne.NewSize(400, 300))
	w.Canvas().Focus(e)

	test.Type(e, "Norway")
	want := []string{
		"root > Europe > Norway",
		"root > Europe > Norway > Bergen",
		"root > Europe > Norway > Oslo",
	}
	if !reflect.DeepEqual(e.Options, want) {
		t.Fatalf("options = %v, want %v", e.Options, want)
	}

	e.OnSubmitted("Norway")
	if q := d.lastQuery(t); !reflect.DeepEqual(q.Ancestors, []string{"no"}) {
		t.Fatalf("query ancestors = %v, want [no]", q.Ancestors)
	}
	if e.Text != "root > Europe > Norway" {
		t.Fatalf("entry text = %q", e.Text)
	}
}

func TestGroupSearch_SubmitWithoutOptionsIgnored(t *testing.T) {
	d := newFakeDelegate(t)
	d.groups = testGroupTree()
	e := newGroupSearchComplete(d)
	test.NewTempWindow(t, e)

	e.OnSubmitted("anything")

	if len(d.searchQueries) != 0 {
		t.Fatalf("expected no search queries, got %v", d.searchQueries)
	}
}
