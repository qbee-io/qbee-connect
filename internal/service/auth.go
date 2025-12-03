package service

import (
	"context"
	"fmt"
	"os"
	
	"go.qbee.io/client"
	"golang.org/x/term"
)

// TerminalLogin handles the CLI based authentication prompt
func TerminalLogin() (*client.Client, error) {
	baseURL := os.Getenv("QBEE_BASEURL")
	if baseURL == "" {
		baseURL = "https://www.app.qbee.io"
	}

	var email string
	fmt.Print("Email: ")
	_, err := fmt.Scanln(&email)
	if err != nil {
		return nil, err
	}

	fmt.Print("Password: ")
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	password := string(bytePassword)
	fmt.Println() 

	cli := client.New().WithBaseURL(baseURL)
	err = cli.Authenticate(context.Background(), email, password)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	config := &client.LoginConfig{
		BaseURL:      baseURL,
		AuthToken:    cli.GetAuthToken(),
		RefreshToken: cli.GetRefreshToken(),
	}
	
	// Assuming LoginWriteConfig is available in your version of client
	err = client.LoginWriteConfig(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to write login config: %w", err)
	}

	return cli, nil
}
