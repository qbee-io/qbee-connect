package components

import (
	"slices"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"

	xwidget "fyne.io/x/fyne/widget"
)

// SearchBarDelegate defines the methods required by the search bar
type SearchBarDelegate interface {
	RefreshUI()
	SetSearchQuery(query client.InventoryListSearch)
	GetAllTags() []string
	GetAllGroups() *client.GroupTree
}

const (
	searchDeviceName = "Device name"
	searchGroup      = "Group"
	searchTag        = "Tag"
)

// NewSearchBar creates a new search bar container
func NewSearchBar(d SearchBarDelegate) *fyne.Container {

	// Create Search Logic (Inline for simplicity or move to components/search.go)
	searchEntry := container.NewVBox()
	searchSelect := widget.NewSelect([]string{searchDeviceName, searchGroup, searchTag}, func(s string) {
		searchEntry.RemoveAll()
		switch s {
		case searchDeviceName:
			e := newDeviceSearchEntry(d)
			searchEntry.Add(e)
		case searchGroup:
			e := newGroupSearchComplete(d)
			searchEntry.Add(e)
		case searchTag:
			// Implement Group search here...
			e := newTagsSearchComplete(d) // Placeholder
			searchEntry.Add(e)
		default:
			return
		}
		// Implement Tag search here...
		searchEntry.Refresh()
	})
	searchSelect.SetSelected(searchDeviceName)

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		d.RefreshUI()
	})

	return container.NewBorder(nil, nil, nil, container.NewHBox(searchSelect, refreshBtn), searchEntry)
}

func newDeviceSearchEntry(d SearchBarDelegate) *widget.Entry {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Search by device name...")
	entry.OnSubmitted = func(val string) {
		d.SetSearchQuery(client.InventoryListSearch{Title: val})
	}
	return entry
}

func newTagsSearchComplete(d SearchBarDelegate) *xwidget.CompletionEntry {

	allTags := d.GetAllTags()

	sort.Strings(allTags)

	tagSearch := xwidget.NewCompletionEntry([]string{})
	tagSearch.SetPlaceHolder("Search for tag..")

	tagSearch.OnChanged = func(search string) {
		filteredOptions := make([]string, 0)

		defer func() {
			tagSearch.SetOptions(filteredOptions)
			tagSearch.ShowCompletion()
		}()

		if strings.TrimSpace(search) == "" {
			filteredOptions = allTags
			return
		}
		for _, option := range allTags {
			if strings.Contains(option, search) {
				filteredOptions = append(filteredOptions, option)
			}
		}
	}

	tagSearch.OnSubmitted = func(selected string) {
		if len(tagSearch.Options) == 0 {
			return
		}

		if strings.TrimSpace(selected) == "" {
			return
		}

		if !slices.Contains(tagSearch.Options, selected) {
			selected = tagSearch.Options[0]
		}

		tagSearch.SetText(selected)
		tagSearch.HideCompletion()
		tagSearch.CursorColumn = len(selected)
		// set search filter to selected tag
		d.SetSearchQuery(client.InventoryListSearch{
			Tags: []string{selected},
		})
	}

	return tagSearch
}

func newGroupSearchComplete(d SearchBarDelegate) *xwidget.CompletionEntry {

	groups := d.GetAllGroups()
	groupMap := getGroupsBreadcrumb(*groups)

	sortedKeys := make([]string, 0, len(groupMap))
	for k := range groupMap {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	groupSearch := xwidget.NewCompletionEntry([]string{})
	groupSearch.SetPlaceHolder("Search for group..")

	groupSearch.OnChanged = func(search string) {
		filteredOptions := make([]string, 0)

		defer func() {
			groupSearch.SetOptions(filteredOptions)
			groupSearch.ShowCompletion()
		}()

		if strings.TrimSpace(search) == "" {
			filteredOptions = sortedKeys
			return
		}

		for _, option := range sortedKeys {
			if strings.Contains(option, search) {
				filteredOptions = append(filteredOptions, option)
			}
		}
	}

	groupSearch.OnSubmitted = func(selected string) {
		if len(groupSearch.Options) == 0 {
			return
		}

		if !slices.Contains(groupSearch.Options, selected) {
			selected = groupSearch.Options[0]
		}

		groupSearch.SetText(selected)
		groupSearch.HideCompletion()
		groupSearch.CursorColumn = len(selected)
		// set search filter to selected group
		d.SetSearchQuery(client.InventoryListSearch{
			Ancestors: []string{groupMap[selected]},
		})
	}

	return groupSearch
}

func getGroupsBreadcrumb(groups client.GroupTree) map[string]string {

	result := make(map[string]string)

	var traverse func(node *client.GroupTreeNode, path []string)
	traverse = func(node *client.GroupTreeNode, path []string) {
		if node == nil {
			return
		}

		newPath := append(path, node.Title)
		for _, child := range node.Nodes {
			if child.Type == client.NodeTypeGroup {
				traverse(&child, newPath)
			}
		}

		if node.Type == client.NodeTypeGroup {
			result[strings.Join(newPath, " > ")] = node.NodeID
			return
		}
	}

	traverse(&groups.Tree, []string{})
	return result
}
