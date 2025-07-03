package coindcx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/b-thark/cdcx-api/pkg/types"
)

// Client represents the CoinDCX API client
type Client struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new CoinDCX client
func NewClient(apiKey, apiSecret string) *Client {
	return &Client{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    "https://api.coindcx.com",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// makeAuthenticatedRequest handles the authenticated API requests
func (c *Client) makeAuthenticatedRequest(endpoint string, requestBody map[string]interface{}) ([]byte, error) {
	requestBody["timestamp"] = time.Now().UnixMilli()

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	signature := c.generateSignature(string(jsonBody))

	url := c.BaseURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AUTH-APIKEY", c.APIKey)
	req.Header.Set("X-AUTH-SIGNATURE", signature)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// makePublicRequest handles public API requests (no authentication needed)
func (c *Client) makePublicRequest(endpoint string) ([]byte, error) {
	url := c.BaseURL + endpoint
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetBalances fetches account balances
func (c *Client) GetBalances() ([]Balance, error) {
	requestBody := make(map[string]interface{})

	responseBody, err := c.makeAuthenticatedRequest("/exchange/v1/users/balances", requestBody)
	if err != nil {
		return nil, err
	}

	var balances []Balance
	if err := json.Unmarshal(responseBody, &balances); err != nil {
		return nil, fmt.Errorf("error parsing balances response: %v", err)
	}

	return balances, nil
}

// GetUserInfo fetches user account information
func (c *Client) GetUserInfo() (*UserInfo, error) {
	requestBody := make(map[string]interface{})

	responseBody, err := c.makeAuthenticatedRequest("/exchange/v1/users/info", requestBody)
	if err != nil {
		return nil, err
	}

	var userInfo UserInfo
	if err := json.Unmarshal(responseBody, &userInfo); err != nil {
		return nil, fmt.Errorf("error parsing user info response: %v", err)
	}

	return &userInfo, nil
}

// GetMarketDetails fetches market details (public endpoint)
func (c *Client) GetMarketDetails() ([]types.MarketDetail, error) {
	responseBody, err := c.makePublicRequest("/exchange/v1/markets_details")
	if err != nil {
		return nil, err
	}

	var markets []types.MarketDetail
	if err := json.Unmarshal(responseBody, &markets); err != nil {
		return nil, fmt.Errorf("error parsing market details response: %v", err)
	}

	return markets, nil
}

// GetTicker fetches ticker data (public endpoint)
func (c *Client) GetTicker() ([]map[string]interface{}, error) {
	responseBody, err := c.makePublicRequest("/exchange/ticker")
	if err != nil {
		return nil, err
	}

	var ticker []map[string]interface{}
	if err := json.Unmarshal(responseBody, &ticker); err != nil {
		return nil, fmt.Errorf("error parsing ticker response: %v", err)
	}

	return ticker, nil
}

// CreateOrder creates a new order
func (c *Client) CreateOrder(orderRequest OrderRequest) (*OrderResponse, error) {
	requestBody := map[string]interface{}{
		"side":           orderRequest.Side,
		"order_type":     orderRequest.OrderType,
		"market":         orderRequest.Market,
		"total_quantity": orderRequest.TotalQuantity,
	}

	// Add price for limit orders
	if orderRequest.OrderType == "limit_order" && orderRequest.PricePerUnit > 0 {
		requestBody["price_per_unit"] = orderRequest.PricePerUnit
	}

	// Add stop price for stop orders
	if orderRequest.StopPrice > 0 {
		requestBody["stop_price"] = orderRequest.StopPrice
	}

	// Add client order ID if provided
	if orderRequest.ClientOrderID != "" {
		requestBody["client_order_id"] = orderRequest.ClientOrderID
	}

	responseBody, err := c.makeAuthenticatedRequest("/exchange/v1/orders/create", requestBody)
	if err != nil {
		return nil, err
	}

	var orderResponse OrderResponse
	if err := json.Unmarshal(responseBody, &orderResponse); err != nil {
		return nil, fmt.Errorf("error parsing order response: %v", err)
	}

	return &orderResponse, nil
}

// GetOrderStatus fetches the status of a specific order
func (c *Client) GetOrderStatus(orderID string) (*Order, error) {
	requestBody := map[string]interface{}{
		"id": orderID,
	}

	responseBody, err := c.makeAuthenticatedRequest("/exchange/v1/orders/status", requestBody)
	if err != nil {
		return nil, err
	}

	var order Order
	if err := json.Unmarshal(responseBody, &order); err != nil {
		return nil, fmt.Errorf("error parsing order status response: %v", err)
	}

	return &order, nil
}

// GetActiveOrders fetches all active orders for a specific market
func (c *Client) GetActiveOrders(market string) ([]Order, error) {
	requestBody := map[string]interface{}{
		"market": market,
	}

	responseBody, err := c.makeAuthenticatedRequest("/exchange/v1/orders/active_orders", requestBody)
	if err != nil {
		return nil, err
	}

	var orders []Order
	if err := json.Unmarshal(responseBody, &orders); err != nil {
		return nil, fmt.Errorf("error parsing active orders response: %v", err)
	}

	return orders, nil
}

// CancelOrder cancels a specific order
func (c *Client) CancelOrder(orderID string) error {
	requestBody := map[string]interface{}{
		"id": orderID,
	}

	_, err := c.makeAuthenticatedRequest("/exchange/v1/orders/cancel", requestBody)
	return err
}
