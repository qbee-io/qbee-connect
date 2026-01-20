package components

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.qbee.io/client"
)

type LoginDialog struct {
	dialog  dialog.Dialog
	onAuth  func(string) error
	authURL string
	window  fyne.Window
}

type loginDelegate interface {
	GetWindow() fyne.Window
	GetClient() *client.Client
	DisplayError(string, string)
	GetContext() context.Context
	SetLoggedIn() error
	RefreshUI()
	PersistLoginConfig() error
}

type pollData struct {
	deviceAuth *client.OAuth2DeviceAuthorizationResponse
	resultChan chan<- error
	stopped    bool
}

var authErr error

func NewLoginDialog(ld loginDelegate) {
	deviceAuth, err := ld.GetClient().OAuth2DeviceAuthorizationRequest(ld.GetContext())
	if err != nil {
		ld.DisplayError("Authentication Error", fmt.Sprintf("failed to initiate device authorization: %v", err))
		return
	}

	parsedURL, err := url.Parse(deviceAuth.VerificationURI)
	if err != nil {
		ld.DisplayError("Authentication Error", fmt.Sprintf("failed to initiate device authorization: %v", err))
		return
	}

	authLink := widget.NewHyperlink(deviceAuth.VerificationURI, parsedURL)
	codeLabel := widget.NewLabel(fmt.Sprintf("User Code: %s", deviceAuth.UserCode))
	loaderLabel := widget.NewLabel("")

	// tick loader

	content := container.NewVBox(
		widget.NewLabel("Click the link to log in:"),
		authLink,
		codeLabel,
		loaderLabel,
	)

	loginDialog := dialog.NewCustom("Authentication Required", "Close", content, ld.GetWindow())

	pollData := &pollData{
		stopped:    false,
		deviceAuth: deviceAuth,
	}

	go func() {
		iter := 0
		for !pollData.stopped {
			iter += 1

			fyne.Do(func() {
				loaderLabel.SetText(fmt.Sprintf("Waiting for authentication%s", strings.Repeat(".", iter%4)))
			})

			time.Sleep(250 * time.Millisecond)
		}
	}()

	err = pollForToken(ld, loginDialog, pollData)
	if err != nil {
		ld.DisplayError("Authentication Error", fmt.Sprintf("failed to obtain token: %v", err))
		return
	}

	loginDialog.SetOnClosed(func() {
		pollData.stopped = true
	})

	loginDialog.Resize(fyne.NewSize(500, 400))
	loginDialog.Show()
}

func pollForToken(ld loginDelegate, loginDialog dialog.Dialog, pollData *pollData) error {

	go func() {

		expirationTime := time.Now().Add(time.Duration(pollData.deviceAuth.ExpiresIn) * time.Second)
		pollingDuration := time.Duration(pollData.deviceAuth.Interval) * time.Second
		for time.Now().Before(expirationTime) && !pollData.stopped {

			time.Sleep(pollingDuration)
			tokenResponse, err := ld.GetClient().OAuth2GetTokenForDeviceCode(ld.GetContext(), pollData.deviceAuth.DeviceCode)

			if errors.Is(err, client.ErrOAuth2AuthorizationPending) {
				continue
			}

			switch err {
			case nil:
				ld.GetClient().WithAuthToken(tokenResponse.AccessToken)
				ld.GetClient().SetRefreshToken(tokenResponse.RefreshToken)

				if err := ld.SetLoggedIn(); err != nil {
					authErr = fmt.Errorf("failed to load user data after authentication: %v", err)
					return
				}

				if err := ld.PersistLoginConfig(); err != nil {
					ld.DisplayError("Authentication Error", fmt.Sprintf("failed to save login configuration: %v", err))
					return
				}
				authErr = nil

			case client.ErrOAuth2BadVerificationCode:
				authErr = fmt.Errorf("invalid verification code provided")
			case client.ErrOAuth2AuthorizationDeclined:
				authErr = fmt.Errorf("authorization was declined by the user")
			case client.ErrOAuth2AuthorizationDeclined:
				authErr = fmt.Errorf("authorization was declined by the user")
			case client.ErrOAuth2ExpiredToken:
				authErr = fmt.Errorf("device code has expired")
			default:
				authErr = fmt.Errorf("error during authentication: %v", err)
			}

			fyne.Do(func() {
				if authErr != nil {
					ld.DisplayError("Authentication Error", authErr.Error())
				}
				loginDialog.Hide()
				ld.RefreshUI()
			})

			return
		}
	}()
	return nil
}
