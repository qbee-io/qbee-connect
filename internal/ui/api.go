package ui

import (
	"fmt"
	"net/http"

	"go.qbee.io/client"
	"go.qbee.io/connect/internal/model"
)

const tagsListPath = "/api/v2/tagslist"

// GetAllGroups fetches all groups from the backend
func (app *App) GetGroups() *client.GroupTree {
	var allGroups *client.GroupTree
	var err error
	app.AuthenticatedRequest(func() error {
		allGroups, err = app.cli.GroupTreeGet(app.ctx, false)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return &client.GroupTree{}
	}
	return allGroups
}

// GetAllTags fetches all tags from the backend
func (app *App) GetTags() []string {
	var allTags []string
	var err error
	app.AuthenticatedRequest(func() error {
		urlParams := make(map[string]string)
		urlParams["scope"] = "nodes"
		urlParams["format"] = "simple"

		tagsListQuery := fmt.Sprintf("%s?scope=%s&format=%s", tagsListPath, urlParams["scope"], urlParams["format"])

		allTags = make([]string, 0)
		if err = app.cli.Call(app.ctx, http.MethodGet, tagsListQuery, nil, &allTags); err != nil {
			return err
		}
		return nil
	})
	return allTags
}

const userDataPath = "/api/v2/user?accounts=true"

// GetUser returns data about the authenticated user
func (app *App) GetUser() (*model.User, error) {
	var user model.User
	err := app.cli.Call(app.ctx, http.MethodGet, userDataPath, nil, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetDevices loads device data from the backend
func (app *App) GetDevices() {
	app.AuthenticatedRequest(func() error {
		app.deviceModel.Query.Offset = app.deviceModel.CurrentPage * app.deviceModel.Query.ItemsPerPage
		devices, err := app.cli.ListDeviceInventory(app.ctx, *app.deviceModel.Query)
		if err != nil {
			return err // Return the error so the wrapper can check for 401
		}
		app.deviceModel.DeviceData = *devices
		return nil
	})
}

const switchAccountPath = "/api/v2/switch-account"

// SwitchAccount changes the current account in use
func (app *App) SwitchAccount(accountID string) error {
	type response struct {
		AuthToken string `json:"token"`
	}

	var err error
	session := new(response)
	app.AuthenticatedRequest(func() error {
		type request struct {
			AccountID string `json:"account_id"`
		}

		authReq := &request{
			AccountID: accountID,
		}

		if err = app.cli.Call(app.ctx, http.MethodPost, switchAccountPath, authReq, session); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	app.cli.WithAuthToken(session.AuthToken)

	if err := app.PersistLoginConfig(); err != nil {
		return err
	}

	user, err := app.GetUser()
	if err != nil {
		return err
	}
	app.user = user

	return nil
}

// AuthenticatedRequest wraps an API call to handle 401 errors globally.
func (app *App) AuthenticatedRequest(apiCall func() error) {
	// 1. Attempt the API call
	err := apiCall()

	// 2. If valid success or non-auth error, stop here
	if err == nil {
		return
	}
	isUnauthorized := false
	clientError, ok := err.(client.Error)
	if ok {
		if code, exists := clientError["code"]; exists {
			if code.(float64) == http.StatusUnauthorized {
				isUnauthorized = true
			}
		}
	}

	// 4. If 401, trigger the Login Dialog
	if isUnauthorized {
		app.SetLoggedOut()
	}
}

// PersistLoginConfig saves the current login configuration to disk
func (app *App) PersistLoginConfig() error {
	clientConfig := client.LoginConfig{
		AuthToken:    app.cli.GetAuthToken(),
		RefreshToken: app.cli.GetRefreshToken(),
		BaseURL:      app.cli.GetBaseURL(),
	}

	if err := client.LoginWriteConfig(clientConfig); err != nil {
		return fmt.Errorf("failed to save login configuration: %v", err)
	}
	return nil
}
