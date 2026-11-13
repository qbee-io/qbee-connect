package model

// User is the logged in user and the accounts they have access to
type User struct {
	Accounts []Account `json:"accounts"`
	User     UserInfo  `json:"user"`
}

// UserInfo holds details about the logged in user
type UserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	AccountID string `json:"account_id"`
	HostName  string `json:"hostname"`
}

// Account is an account the user has access to
type Account struct {
	ID    string `json:"account_id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
