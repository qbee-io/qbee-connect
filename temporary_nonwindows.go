//go:build unix || darwin

package main

import (
	"context"
	"fmt"
	"os"

	"go.qbee.io/client"
	"golang.org/x/term"
)

// Temporary terminal login function for testing purposes until login with backends is implemented.
func promptUsernamePassword() (string, string, error) {
	var email string
	fmt.Print("Email: ")
	_, err := fmt.Scanln(&email)
	if err != nil {
		return "", "", err
	}

	fmt.Print("Password: ")
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", "", err
	}
	password := string(bytePassword)
	fmt.Println() // Move to next line after password input

	return email, password, nil
}

func terminalLogin() (*client.Client, error) {
	baseURL := os.Getenv("QBEE_BASEURL")
	if baseURL == "" {
		baseURL = "https://www.app.qbee.io"
	}

	username, password, err := promptUsernamePassword()
	if err != nil {
		return nil, fmt.Errorf("failed to read username/password: %w", err)
	}

	cli := client.New().WithBaseURL(baseURL)
	err = cli.Authenticate(context.Background(), username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate qbee client: %w", err)
	}

	config := &client.LoginConfig{
		BaseURL:      baseURL,
		AuthToken:    cli.GetAuthToken(),
		RefreshToken: cli.GetRefreshToken(),
	}
	err = client.LoginWriteConfig(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to write login config: %w", err)
	}

	return cli, nil
}
