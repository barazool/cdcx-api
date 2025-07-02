package coindcx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents the CoinDCX API client
type Client struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	PublicURL  string
	HTTPClient *http.Client
}

// APIError represents an error response from the API
type APIError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
	Status  int    `json:"status"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API Error %d: %s", e.Code, e.Message)
}

// NewClient creates a new CoinDCX client
func NewClient(apiKey, apiSecret string) *Client {
	return &Client{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    "https://api.coindcx.com",
		PublicURL:  "https://public.coindcx.com",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// makePublicRequest handles public API requests (no authentication needed)
func (c *Client) makePublicRequest(endpoint string) ([]byte, error) {
	url := c.BaseURL + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

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

// makePublicRequestWithCustomURL handles public API requests with custom URL
func (c *Client) makePublicRequestWithCustomURL(baseURL, endpoint string) ([]byte, error) {
	url := baseURL + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

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
		// Try to parse as API error
		var apiErr APIError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
			return nil, &apiErr
		}
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

	// According to API docs, user info returns an array
	var userInfoArray []UserInfo
	if err := json.Unmarshal(responseBody, &userInfoArray); err != nil {
		return nil, fmt.Errorf("error parsing user info response: %v", err)
	}

	if len(userInfoArray) == 0 {
		return nil, fmt.Errorf("no user info returned")
	}

	return &userInfoArray[0], nil
}

// GetTicker fetches ticker data for all markets
func (c *Client) GetTicker() ([]Ticker, error) {
	responseBody, err := c.makePublicRequest("/exchange/ticker")
	if err != nil {
		return nil, err
	}

	var tickers []Ticker
	if err := json.Unmarshal(responseBody, &tickers); err != nil {
		return nil, fmt.Errorf("error parsing ticker response: %v", err)
	}

	return tickers, nil
}

// GetMarkets fetches all available markets
func (c *Client) GetMarkets() ([]string, error) {
	responseBody, err := c.makePublicRequest("/exchange/v1/markets")
	if err != nil {
		return nil, err
	}

	var markets []string
	if err := json.Unmarshal(responseBody, &markets); err != nil {
		return nil, fmt.Errorf("error parsing markets response: %v", err)
	}

	return markets, nil
}

// GetMarketsDetails fetches detailed information for all markets
func (c *Client) GetMarketsDetails() ([]MarketDetail, error) {
	responseBody, err := c.makePublicRequest("/exchange/v1/markets_details")
	if err != nil {
		return nil, err
	}

	var marketsDetails []MarketDetail
	if err := json.Unmarshal(responseBody, &marketsDetails); err != nil {
		return nil, fmt.Errorf("error parsing markets details response: %v", err)
	}

	return marketsDetails, nil
}

// GetOrderBook fetches order book for a specific pair
func (c *Client) GetOrderBook(pair string) (*OrderBook, error) {
	endpoint := fmt.Sprintf("/market_data/orderbook?pair=%s", pair)
	responseBody, err := c.makePublicRequestWithCustomURL(c.PublicURL, endpoint)
	if err != nil {
		return nil, err
	}

	var orderBook OrderBook
	if err := json.Unmarshal(responseBody, &orderBook); err != nil {
		return nil, fmt.Errorf("error parsing orderbook response: %v", err)
	}

	return &orderBook, nil
}

// GetTradeHistory fetches trade history for a specific pair
func (c *Client) GetTradeHistory(pair string, limit int) ([]Trade, error) {
	endpoint := fmt.Sprintf("/market_data/trade_history?pair=%s&limit=%d", pair, limit)
	responseBody, err := c.makePublicRequestWithCustomURL(c.PublicURL, endpoint)
	if err != nil {
		return nil, err
	}

	var trades []Trade
	if err := json.Unmarshal(responseBody, &trades); err != nil {
		return nil, fmt.Errorf("error parsing trade history response: %v", err)
	}

	return trades, nil
}
