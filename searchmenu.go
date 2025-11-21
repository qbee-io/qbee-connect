package main

import (
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
)

const (
	searchByDeviceName = "Device name"
	searchByGroup      = "Group"
	searchByTag        = "Tag"
)

func (app *App) setupSearchBySelect(searchEntry *fyne.Container) *widget.Select {
	options := []string{
		string(searchByDeviceName),
		string(searchByGroup),
		string(searchByTag),
	}
	selectWidget := widget.NewSelect(options, func(selected string) {
		searchEntry.RemoveAll()

		switch selected {
		case searchByDeviceName, searchByTag:
			textSearch := app.newTextSearchField(selected)
			searchEntry.Add(textSearch)
		case searchByGroup:
			groupSearch := app.newGroupSearchSelect()
			if groupSearch == nil {
				return
			}
			searchEntry.Add(groupSearch)
		default:
			searchType := searchByDeviceName
			textSearch := app.newTextSearchField(searchType)
			searchEntry.Add(textSearch)
		}

		searchEntry.Refresh()
	})
	return selectWidget
}

func (app *App) newTextSearchField(searchType string) *widget.Entry {

	textSearch := widget.NewEntry()
	textSearch.SetPlaceHolder("Search by " + searchType)

	setSearch := func(searchType, searchString string) {
		var search client.InventoryListSearch
		if searchType == searchByTag {
			// Need to search by exact match or with * suffix
			search = client.InventoryListSearch{
				Tags: []string{searchString},
			}
		} else {
			search = client.InventoryListSearch{
				Title: searchString,
			}
		}
		app.deviceModel.currentPage = 0
		app.deviceModel.query.Search = search
	}

	textSearch.OnSubmitted = func(searchTerm string) {
		setSearch(searchType, searchTerm)

		// tag searches are exact matches unless wildcard is used and they need to
		// have a minimum of 3 characters
		if searchType == searchByTag && len(searchTerm) < 3 {
			return
		}

		err := app.refreshDeviceListUI()
		if err != nil {
			app.fyneApp.SendNotification(&fyne.Notification{
				Title:   "Device Fetch Error",
				Content: "Failed to load devices for search: " + err.Error(),
			})
		}
	}

	setSearch(searchType, "")

	// Do not refresh device list on initial setup for tag search as it requires
	// minimum 3 characters
	if searchType == searchByTag {
		return textSearch
	}

	err := app.refreshDeviceListUI()
	if err != nil {
		app.fyneApp.SendNotification(&fyne.Notification{
			Title:   "Device Fetch Error",
			Content: "Failed to load devices for selected group: " + err.Error(),
		})
	}
	return textSearch
}

func (app *App) newGroupSearchSelect() *widget.Select {
	allGroups, err := app.cli.GroupTreeGet(app.ctx, false)

	if err != nil {
		app.fyneApp.SendNotification(&fyne.Notification{
			Title:   "Group Fetch Error",
			Content: "Failed to load groups for search dropdown: " + err.Error(),
		})
		return nil
	}
	groupMap := app.getGroupsBreadcrumb(*allGroups)

	groupBreadCrumbs := make([]string, 0, len(groupMap))
	for k := range groupMap {
		groupBreadCrumbs = append(groupBreadCrumbs, k)
	}

	sort.Strings(groupBreadCrumbs)

	groupDropdown := widget.NewSelect(groupBreadCrumbs, func(selected string) {
		// set search filter to selected group
		app.deviceModel.query.Search = client.InventoryListSearch{
			Ancestors: []string{groupMap[selected]},
		}
		app.deviceModel.currentPage = 0
		err := app.refreshDeviceListUI()
		if err != nil {
			app.fyneApp.SendNotification(&fyne.Notification{
				Title:   "Device Fetch Error",
				Content: "Failed to load devices for selected group: " + err.Error(),
			})
		}
	})
	groupDropdown.SetSelectedIndex(0)

	return groupDropdown
}
