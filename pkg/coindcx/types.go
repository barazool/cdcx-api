package coindcx

// Balance represents account balance for a currency
type Balance struct {
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
	Locked   float64 `json:"locked_balance"`
}

// UserInfo represents user account information
type UserInfo struct {
	CoinDCXID    string `json:"coindcx_id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	MobileNumber string `json:"mobile_number"`
	Email        string `json:"email"`
}
