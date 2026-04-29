package model

type User struct {
	Accounts []Account `json:"accounts"`
	User     UserInfo  `json:"user"`
}

type UserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	AccountID string `json:"account_id"`
	HostName  string `json:"hostname"`
}

type Account struct {
	ID    string `json:"account_id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
