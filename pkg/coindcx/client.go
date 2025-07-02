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
