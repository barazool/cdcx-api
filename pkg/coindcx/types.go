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

// Ticker represents market ticker data
type Ticker struct {
	Market       string `json:"market"`
	Change24Hour string `json:"change_24_hour"`
	High         string `json:"high"`
	Low          string `json:"low"`
	Volume       string `json:"volume"`
	LastPrice    string `json:"last_price"`
	Bid          string `json:"bid"`
	Ask          string `json:"ask"`
	Timestamp    int64  `json:"timestamp"`
}

// MarketDetail represents detailed market information
type MarketDetail struct {
	CoinDCXName             string   `json:"coindcx_name"`
	BaseCurrencyShortName   string   `json:"base_currency_short_name"`
	TargetCurrencyShortName string   `json:"target_currency_short_name"`
	TargetCurrencyName      string   `json:"target_currency_name"`
	BaseCurrencyName        string   `json:"base_currency_name"`
	MinQuantity             float64  `json:"min_quantity"`
	MaxQuantity             float64  `json:"max_quantity"`
	MinPrice                float64  `json:"min_price"`
	MaxPrice                float64  `json:"max_price"`
	MinNotional             float64  `json:"min_notional"`
	BaseCurrencyPrecision   int      `json:"base_currency_precision"`
	TargetCurrencyPrecision int      `json:"target_currency_precision"`
	Step                    float64  `json:"step"`
	OrderTypes              []string `json:"order_types"`
	Symbol                  string   `json:"symbol"`
	Ecode                   string   `json:"ecode"`
	MaxLeverage             float64  `json:"max_leverage"`
	MaxLeverageShort        *float64 `json:"max_leverage_short"`
	Pair                    string   `json:"pair"`
	Status                  string   `json:"status"`
}

// OrderBook represents order book data
type OrderBook struct {
	Bids map[string]string `json:"bids"`
	Asks map[string]string `json:"asks"`
}

// Trade represents trade history data
type Trade struct {
	Price     float64 `json:"p"`
	Quantity  float64 `json:"q"`
	Symbol    string  `json:"s"`
	Timestamp int64   `json:"T"`
	IsMaker   bool    `json:"m"`
}
