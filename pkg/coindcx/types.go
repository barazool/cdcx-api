package coindcx

import (
	"encoding/json"
	"strconv"
)

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

// OrderRequest represents a request to create an order
type OrderRequest struct {
	Side          string  `json:"side"`                      // "buy" or "sell"
	OrderType     string  `json:"order_type"`                // "market_order" or "limit_order"
	Market        string  `json:"market"`                    // e.g., "BTCINR"
	TotalQuantity float64 `json:"total_quantity"`            // Amount to trade
	PricePerUnit  float64 `json:"price_per_unit,omitempty"`  // Price for limit orders
	StopPrice     float64 `json:"stop_price,omitempty"`      // Stop price for stop orders
	ClientOrderID string  `json:"client_order_id,omitempty"` // Optional client order ID
	Timestamp     int64   `json:"timestamp"`                 // Unix timestamp in milliseconds
}

// FlexibleTimestamp handles both string and int timestamps
type FlexibleTimestamp string

func (ft *FlexibleTimestamp) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*ft = FlexibleTimestamp(s)
		return nil
	}

	// If that fails, try as number
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		*ft = FlexibleTimestamp(strconv.FormatInt(n, 10))
		return nil
	}

	// If both fail, try as float
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		*ft = FlexibleTimestamp(strconv.FormatFloat(f, 'f', 0, 64))
		return nil
	}

	return json.Unmarshal(data, (*string)(ft))
}

// Order represents an order returned by the API
type Order struct {
	ID                string            `json:"id"`
	ClientOrderID     string            `json:"client_order_id,omitempty"`
	Market            string            `json:"market"`
	OrderType         string            `json:"order_type"`
	Side              string            `json:"side"`
	Status            string            `json:"status"`
	FeeAmount         float64           `json:"fee_amount"`
	Fee               float64           `json:"fee"`
	TotalQuantity     float64           `json:"total_quantity"`
	RemainingQuantity float64           `json:"remaining_quantity"`
	AvgPrice          float64           `json:"avg_price"`
	PricePerUnit      float64           `json:"price_per_unit"`
	CreatedAt         FlexibleTimestamp `json:"created_at"`
	UpdatedAt         FlexibleTimestamp `json:"updated_at"`
}

// OrderResponse represents the response when creating an order
type OrderResponse struct {
	Orders []Order `json:"orders"`
}
