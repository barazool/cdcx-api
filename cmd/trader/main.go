package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Client represents the CoinDCX API client
type Client struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	HTTPClient *http.Client
}

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

// NewClient creates a new CoinDCX client
func NewClient() (*Client, error) {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	apiKey := os.Getenv("COINDCX_API_KEY")
	apiSecret := os.Getenv("COINDCX_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("COINDCX_API_KEY and COINDCX_API_SECRET must be set in .env file")
	}

	return &Client{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    "https://api.coindcx.com",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// generateSignature creates HMAC-SHA256 signature for authentication
func (c *Client) generateSignature(payload string) string {
	h := hmac.New(sha256.New, []byte(c.APISecret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// makeAuthenticatedRequest handles the authenticated API requests
func (c *Client) makeAuthenticatedRequest(endpoint string, requestBody map[string]interface{}) ([]byte, error) {
	// Add timestamp to request body
	requestBody["timestamp"] = time.Now().UnixMilli()

	// Convert to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	// Generate signature
	signature := c.generateSignature(string(jsonBody))

	// Create HTTP request
	url := c.BaseURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AUTH-APIKEY", c.APIKey)
	req.Header.Set("X-AUTH-SIGNATURE", signature)

	// Make request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Check status code
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

	// First try parsing as array (in case API returns array)
	var userInfoArray []UserInfo
	if err := json.Unmarshal(responseBody, &userInfoArray); err == nil && len(userInfoArray) > 0 {
		return &userInfoArray[0], nil
	}

	// If array parsing fails, try parsing as single object
	var userInfo UserInfo
	if err := json.Unmarshal(responseBody, &userInfo); err != nil {
		return nil, fmt.Errorf("error parsing user info response: %v. Raw response: %s", err, string(responseBody))
	}

	return &userInfo, nil
}

func main() {
	// Create client
	client, err := NewClient()
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("CoinDCX API Client - Testing Account Details")
	fmt.Println("==========================================")

	// Get user info
	fmt.Println("\n1. Fetching User Info...")
	userInfo, err := client.GetUserInfo()
	if err != nil {
		fmt.Printf("Error fetching user info: %v\n", err)
	} else {
		fmt.Printf("✅ User ID: %s\n", userInfo.CoinDCXID)
		fmt.Printf("   Name: %s %s\n", userInfo.FirstName, userInfo.LastName)
		fmt.Printf("   Email: %s\n", userInfo.Email)
	}

	// Get balances
	fmt.Println("\n2. Fetching Account Balances...")
	balances, err := client.GetBalances()
	if err != nil {
		fmt.Printf("Error fetching balances: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d currency balances:\n", len(balances))

		// Show only non-zero balances
		for _, balance := range balances {
			if balance.Balance > 0 || balance.Locked > 0 {
				fmt.Printf("   %s: %.8f (Locked: %.8f)\n",
					balance.Currency, balance.Balance, balance.Locked)
			}
		}
	}
}
