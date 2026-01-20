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
	//resultChan := make(chan error)
	// create and input field for the base URL
	baseURLInput := widget.NewEntry()
	baseURLInput.SetPlaceHolder(ld.GetClient().GetBaseURL())
	baseURLInput.Text = ld.GetClient().GetBaseURL() // Default

	// Create hyperlink for OAuth2 authentication
	authLink := widget.NewHyperlink("Click here to authenticate", nil)
	codeLabel := widget.NewLabel("")
	loaderLabel := widget.NewLabel("")

	var loginDialog dialog.Dialog

	pollData := &pollData{
		stopped: false,
	}

	step2 := container.NewVBox(
		authLink,
		codeLabel,
		loaderLabel,
	)

	step2.Hide()

	var generateCodeBtn *widget.Button

	generateCodeBtn = widget.NewButton("Generate Device Code", func() {
		ld.GetClient().WithBaseURL(baseURLInput.Text)
		deviceAuth, err := ld.GetClient().OAuth2DeviceAuthorizationRequest(ld.GetContext())
		if err != nil {
			ld.DisplayError("Authentication Error", fmt.Sprintf("failed to initiate device authorization: %v", err))
			return
		}

		if parsedURL, err := url.Parse(deviceAuth.VerificationURI); err == nil {
			authLink.SetURL(parsedURL)
		}

		generateCodeBtn.Disable()
		codeLabel.SetText(fmt.Sprintf("User Code: %s", deviceAuth.UserCode))

		// tick loader
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

		step2.Show()
		// Start polling for token
		pollData.deviceAuth = deviceAuth
		err = pollForToken(ld, loginDialog, pollData)
		if err != nil {
			ld.DisplayError("Authentication Error", fmt.Sprintf("failed to obtain token: %v", err))
			return
		}
	})

	// Layout
	content := container.NewVBox(
		widget.NewLabel("Please authenticate to continue."),
		widget.NewLabel("Enter the Base URL of your Qbee.io instance:"),
		baseURLInput,
		generateCodeBtn,
		step2,
	)

	loginDialog = dialog.NewCustom("Authentication Required", "Close", content, ld.GetWindow())
	loginDialog.SetOnClosed(func() {
		// stop any active polling or cleanup if necessary
		pollData.stopped = true
		//resultChan <- authErr

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
