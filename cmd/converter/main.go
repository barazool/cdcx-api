package main

/*
 * sometimes we run into:
 * A different minimum for market orders vs limit orders
 A minimum notional value (total INR amount) that's higher than what you're trying to spend
 The MinNotional field - which you should also check. This represents the minimum total value of the trade in INR.

 You should check both:

 MinQuantity (minimum USDT amount)
 MinNotional (minimum INR value of the trade)

 The MinNotional is probably what's causing your issue - even though 1.13 USDT meets the quantity requirement, the total trade value (₹99.89) might be below the minimum notional value required by the exchange.
 so validate both before placing order
 * OUTPUT:
 * 📊 Step 1: Checking current balances...
 💰 Current INR Balance: ₹316.30
 💰 Current USDT Balance: 120.63813491 USDT

 📋 Step 2: Checking USDTINR market details...
 ✅ Market Details:
    Min Quantity: 0.01000000 USDT
    Target Precision: 2 decimals

 💱 Step 3: Getting current USDT price...
 💰 Current USDT Price: ₹88.44

 🧮 Step 4: Calculation:
    Converting: ₹120.00 INR
    USDT Price: ₹88.44
    Estimated USDT: 1.35685210
    Rounded USDT (2 decimals): 1.35

 ⚠️  TRANSACTION CONFIRMATION
 You are about to:
 • BUY 1.35 USDT
 • Using approximately ₹119.39 INR
 • At market price (current: ₹88.44 per USDT)
 • Remaining INR after trade: ₹196.91

 Note: This is a MARKET ORDER - the actual price may vary slightly
 due to market movements and will be executed at the best available price.

 Type 'YES' to confirm this transaction: YES

 🚀 Step 6: Executing market buy order...
 ✅ Order created successfully!
    Order ID: b007f77a-57fa-11f0-8f88-3f062fc6e5ed
    Status: open
    Market: USDTINR
    Side: buy
    Type: market_order
    Quantity: 1.35 USDT

 ⏳ Waiting 3 seconds for order processing...

 📊 Final Step: Checking updated balances...
 💰 Updated Balances:
    INR: ₹196.14 (was ₹316.30)
    USDT: 121.98813491 (was 120.63813491)
    USDT Gained: +1.35000000
    INR Spent: ₹120.17

 🎉 Transaction completed successfully!
 💡 You converted ₹120.00 to USDT and still have ₹196.30 remaining!
 *
*/
import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
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

// MarketDetail represents market information
type MarketDetail struct {
	CoindcxName             string   `json:"coindcx_name"`
	BaseCurrencyShortName   string   `json:"base_currency_short_name"`
	TargetCurrencyShortName string   `json:"target_currency_short_name"`
	MinQuantity             float64  `json:"min_quantity"`
	MaxQuantity             float64  `json:"max_quantity"`
	MinPrice                float64  `json:"min_price"`
	MaxPrice                float64  `json:"max_price"`
	MinNotional             float64  `json:"min_notional"`
	BaseCurrencyPrecision   int      `json:"base_currency_precision"`
	TargetCurrencyPrecision int      `json:"target_currency_precision"`
	Step                    float64  `json:"step"`
	OrderTypes              []string `json:"order_types"`
	Status                  string   `json:"status"`
}

// OrderRequest represents an order creation request
type OrderRequest struct {
	Side          string  `json:"side"`
	OrderType     string  `json:"order_type"`
	Market        string  `json:"market"`
	PricePerUnit  float64 `json:"price_per_unit,omitempty"`
	TotalQuantity float64 `json:"total_quantity"`
	Timestamp     int64   `json:"timestamp"`
}

// OrderResponse represents the response from order creation
type OrderResponse struct {
	Orders []Order `json:"orders"`
}

type Order struct {
	ID                string  `json:"id"`
	Market            string  `json:"market"`
	OrderType         string  `json:"order_type"`
	Side              string  `json:"side"`
	Status            string  `json:"status"`
	FeeAmount         float64 `json:"fee_amount"`
	Fee               float64 `json:"fee"`
	TotalQuantity     float64 `json:"total_quantity"`
	RemainingQuantity float64 `json:"remaining_quantity"`
	AvgPrice          float64 `json:"avg_price"`
	PricePerUnit      float64 `json:"price_per_unit"`
	CreatedAt         int64   `json:"created_at"`
	UpdatedAt         int64   `json:"updated_at"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Printf("❌ Error loading .env file: %v\n", err)
		os.Exit(1)
	}

	apiKey := os.Getenv("COINDCX_API_KEY")
	apiSecret := os.Getenv("COINDCX_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		fmt.Printf("❌ COINDCX_API_KEY and COINDCX_API_SECRET must be set in .env file\n")
		os.Exit(1)
	}

	client := &Client{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    "https://api.coindcx.com",
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	fmt.Println("💰 Convert ₹120 INR to USDT")
	fmt.Println("===============================")

	// Step 1: Get current balances
	fmt.Println("\n📊 Step 1: Checking current balances...")
	balances, err := client.GetBalances()
	if err != nil {
		fmt.Printf("❌ Error fetching balances: %v\n", err)
		os.Exit(1)
	}

	var inrBalance float64
	var usdtBalance float64
	for _, balance := range balances {
		if balance.Currency == "INR" {
			inrBalance = balance.Balance
		}
		if balance.Currency == "USDT" {
			usdtBalance = balance.Balance
		}
	}

	fmt.Printf("💰 Current INR Balance: ₹%.2f\n", inrBalance)
	fmt.Printf("💰 Current USDT Balance: %.8f USDT\n", usdtBalance)

	// Fixed amount to convert
	const CONVERT_AMOUNT = 120.0

	if inrBalance < CONVERT_AMOUNT {
		fmt.Printf("❌ Insufficient INR balance. Need ₹%.2f, have ₹%.2f\n", CONVERT_AMOUNT, inrBalance)
		os.Exit(1)
	}

	// Step 2: Check market details for USDTINR
	fmt.Println("\n📋 Step 2: Checking USDTINR market details...")
	marketDetails, err := client.GetMarketDetails()
	if err != nil {
		fmt.Printf("❌ Error fetching market details: %v\n", err)
		os.Exit(1)
	}

	var usdtinrMarket *MarketDetail
	for _, market := range marketDetails {
		if market.CoindcxName == "USDTINR" {
			usdtinrMarket = &market
			break
		}
	}

	if usdtinrMarket == nil {
		fmt.Printf("❌ USDTINR market not found\n")
		os.Exit(1)
	}

	if usdtinrMarket.Status != "active" {
		fmt.Printf("❌ USDTINR market is not active. Status: %s\n", usdtinrMarket.Status)
		os.Exit(1)
	}

	fmt.Printf("✅ Market Details:\n")
	fmt.Printf("   Min Quantity: %.8f USDT\n", usdtinrMarket.MinQuantity)
	fmt.Printf("   Target Precision: %d decimals\n", usdtinrMarket.TargetCurrencyPrecision)

	// Step 3: Get current market price
	fmt.Println("\n💱 Step 3: Getting current USDT price...")
	ticker, err := client.GetTicker()
	if err != nil {
		fmt.Printf("❌ Error fetching ticker: %v\n", err)
		os.Exit(1)
	}

	var usdtPrice float64
	var found bool
	for _, tick := range ticker {
		if market, ok := tick["market"].(string); ok && market == "USDTINR" {
			if lastPriceStr, ok := tick["last_price"].(string); ok {
				if price, err := strconv.ParseFloat(lastPriceStr, 64); err == nil {
					usdtPrice = price
					found = true
					fmt.Printf("💰 Current USDT Price: ₹%.2f\n", usdtPrice)
					break
				}
			}
		}
	}

	if !found || usdtPrice == 0 {
		fmt.Printf("❌ Could not fetch USDT price from ticker\n")
		os.Exit(1)
	}

	// Step 4: Calculate how much USDT we can buy with ₹10,500
	estimatedUSDT := CONVERT_AMOUNT / usdtPrice

	// Round USDT to required precision (target_currency_precision)
	precisionFactor := math.Pow(10, float64(usdtinrMarket.TargetCurrencyPrecision))
	roundedUSDT := math.Floor(estimatedUSDT*precisionFactor) / precisionFactor

	fmt.Printf("\n🧮 Step 4: Calculation:\n")
	fmt.Printf("   Converting: ₹%.2f INR\n", CONVERT_AMOUNT)
	fmt.Printf("   USDT Price: ₹%.2f\n", usdtPrice)
	fmt.Printf("   Estimated USDT: %.8f\n", estimatedUSDT)
	fmt.Printf("   Rounded USDT (%d decimals): %.*f\n",
		usdtinrMarket.TargetCurrencyPrecision,
		usdtinrMarket.TargetCurrencyPrecision,
		roundedUSDT)

	// Check if we meet minimum requirements
	if roundedUSDT < usdtinrMarket.MinQuantity {
		fmt.Printf("❌ Calculated USDT quantity (%.*f) is below minimum (%.8f)\n",
			usdtinrMarket.TargetCurrencyPrecision, roundedUSDT, usdtinrMarket.MinQuantity)
		os.Exit(1)
	}

	actualINRRequired := roundedUSDT * usdtPrice
	if actualINRRequired > CONVERT_AMOUNT {
		fmt.Printf("❌ Rounded USDT amount exceeds budget. Need ₹%.2f, budget ₹%.2f\n",
			actualINRRequired, CONVERT_AMOUNT)
		os.Exit(1)
	}

	// Step 5: Confirm transaction
	fmt.Printf("\n⚠️  TRANSACTION CONFIRMATION\n")
	fmt.Printf("You are about to:\n")
	fmt.Printf("• BUY %.*f USDT\n", usdtinrMarket.TargetCurrencyPrecision, roundedUSDT)
	fmt.Printf("• Using approximately ₹%.2f INR\n", actualINRRequired)
	fmt.Printf("• At market price (current: ₹%.2f per USDT)\n", usdtPrice)
	fmt.Printf("• Remaining INR after trade: ₹%.2f\n", inrBalance-actualINRRequired)
	fmt.Printf("\nNote: This is a MARKET ORDER - the actual price may vary slightly\n")
	fmt.Printf("due to market movements and will be executed at the best available price.\n\n")

	fmt.Print("Type 'YES' to confirm this transaction: ")
	var confirmation string
	fmt.Scanln(&confirmation)

	if confirmation != "YES" {
		fmt.Println("❌ Transaction cancelled by user")
		os.Exit(0)
	}

	// Step 6: Execute the market buy order
	fmt.Println("\n🚀 Step 6: Executing market buy order...")

	orderRequest := OrderRequest{
		Side:          "buy",
		OrderType:     "market_order",
		Market:        "USDTINR",
		TotalQuantity: roundedUSDT,
		Timestamp:     time.Now().UnixMilli(),
	}

	orderResponse, err := client.CreateOrder(orderRequest)
	if err != nil {
		fmt.Printf("❌ Error creating order: %v\n", err)
		os.Exit(1)
	}

	if len(orderResponse.Orders) == 0 {
		fmt.Printf("❌ No order was created\n")
		os.Exit(1)
	}

	order := orderResponse.Orders[0]
	fmt.Printf("✅ Order created successfully!\n")
	fmt.Printf("   Order ID: %s\n", order.ID)
	fmt.Printf("   Status: %s\n", order.Status)
	fmt.Printf("   Market: %s\n", order.Market)
	fmt.Printf("   Side: %s\n", order.Side)
	fmt.Printf("   Type: %s\n", order.OrderType)
	fmt.Printf("   Quantity: %.*f USDT\n", usdtinrMarket.TargetCurrencyPrecision, order.TotalQuantity)

	// Step 7: Wait a moment and check updated balances
	fmt.Println("\n⏳ Waiting 3 seconds for order processing...")
	time.Sleep(3 * time.Second)

	fmt.Println("\n📊 Final Step: Checking updated balances...")
	newBalances, err := client.GetBalances()
	if err != nil {
		fmt.Printf("❌ Error fetching updated balances: %v\n", err)
	} else {
		var newINRBalance, newUSDTBalance float64
		for _, balance := range newBalances {
			if balance.Currency == "INR" {
				newINRBalance = balance.Balance
			}
			if balance.Currency == "USDT" {
				newUSDTBalance = balance.Balance
			}
		}

		fmt.Printf("💰 Updated Balances:\n")
		fmt.Printf("   INR: ₹%.2f (was ₹%.2f)\n", newINRBalance, inrBalance)
		fmt.Printf("   USDT: %.8f (was %.8f)\n", newUSDTBalance, usdtBalance)
		fmt.Printf("   USDT Gained: +%.8f\n", newUSDTBalance-usdtBalance)
		fmt.Printf("   INR Spent: ₹%.2f\n", inrBalance-newINRBalance)
	}

	fmt.Println("\n🎉 Transaction completed successfully!")
	fmt.Printf("💡 You converted ₹%.2f to USDT and still have ₹%.2f remaining!\n",
		CONVERT_AMOUNT, inrBalance-CONVERT_AMOUNT)
}

// Client methods
func (c *Client) generateSignature(payload string) string {
	h := hmac.New(sha256.New, []byte(c.APISecret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

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

func (c *Client) GetMarketDetails() ([]MarketDetail, error) {
	responseBody, err := c.makePublicRequest("/exchange/v1/markets_details")
	if err != nil {
		return nil, err
	}

	var marketDetails []MarketDetail
	if err := json.Unmarshal(responseBody, &marketDetails); err != nil {
		return nil, fmt.Errorf("error parsing market details response: %v", err)
	}

	return marketDetails, nil
}

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

func (c *Client) CreateOrder(order OrderRequest) (*OrderResponse, error) {
	requestBody := map[string]interface{}{
		"side":           order.Side,
		"order_type":     order.OrderType,
		"market":         order.Market,
		"total_quantity": order.TotalQuantity,
		"timestamp":      order.Timestamp,
	}

	// Only add price for limit orders
	if order.OrderType == "limit_order" {
		requestBody["price_per_unit"] = order.PricePerUnit
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
