package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// MarketDetail represents market information from CoinDCX
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
	ECode                   string   `json:"ecode"`
	MaxLeverage             *float64 `json:"max_leverage"`
	MaxLeverageShort        *float64 `json:"max_leverage_short"`
	Pair                    string   `json:"pair"`
	Status                  string   `json:"status"`
}

// PairInfo stores essential pair information for arbitrage
type PairInfo struct {
	Symbol         string  `json:"symbol"`          // RENDERINR, RENDERUSDT
	Pair           string  `json:"pair"`            // B-RENDER_INR, B-RENDER_USDT
	BaseCurrency   string  `json:"base_currency"`   // INR, USDT
	TargetCurrency string  `json:"target_currency"` // RENDER
	MinQuantity    float64 `json:"min_quantity"`
	MinNotional    float64 `json:"min_notional"`
	Status         string  `json:"status"`
}

// USDTArbitragePairs stores USDT-based arbitrage opportunities
type USDTArbitragePairs struct {
	TargetCurrency string     `json:"target_currency"`
	USDTPair       PairInfo   `json:"usdt_pair"`   // The USDT pair to buy from
	OtherPairs     []PairInfo `json:"other_pairs"` // Other pairs to sell to
	LastUpdated    time.Time  `json:"last_updated"`
}

func main() {
	fmt.Println("🔍 CoinDCX USDT-Based Arbitrage Pair Fetcher")
	fmt.Println("=============================================")
	fmt.Println("🎯 Focus: USDT → Other Currency arbitrage opportunities")

	// Fetch market details
	fmt.Println("\n📊 Fetching market details...")
	markets, err := fetchMarketDetails()
	if err != nil {
		fmt.Printf("❌ Error fetching markets: %v\n", err)
		return
	}
	fmt.Printf("✅ Found %d total markets\n", len(markets))

	// Extract USDT-based arbitrage pairs
	usdtArbitragePairs := extractUSDTArbitragePairs(markets)

	// Save to file
	err = saveUSDTArbitragePairs(usdtArbitragePairs, "usdt_arbitrage_pairs.json")
	if err != nil {
		fmt.Printf("❌ Error saving pairs: %v\n", err)
		return
	}

	// Display USDT arbitrage opportunities
	displayUSDTArbitrageOpportunities(usdtArbitragePairs)
}

func fetchMarketDetails() ([]MarketDetail, error) {
	url := "https://api.coindcx.com/exchange/v1/markets_details"

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var markets []MarketDetail
	if err := json.Unmarshal(body, &markets); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}
	return markets, nil
}

func extractUSDTArbitragePairs(markets []MarketDetail) map[string]USDTArbitragePairs {
	// First, group all pairs by target currency
	allPairs := make(map[string][]PairInfo)

	for _, market := range markets {
		// Only include active markets
		if market.Status != "active" {
			continue
		}

		targetCurrency := market.TargetCurrencyShortName

		pairInfo := PairInfo{
			Symbol:         market.Symbol,
			Pair:           market.Pair,
			BaseCurrency:   market.BaseCurrencyShortName,
			TargetCurrency: targetCurrency,
			MinQuantity:    market.MinQuantity,
			MinNotional:    market.MinNotional,
			Status:         market.Status,
		}

		allPairs[targetCurrency] = append(allPairs[targetCurrency], pairInfo)
	}

	// Now extract only those currencies that have USDT pair + other pairs
	usdtArbitragePairs := make(map[string]USDTArbitragePairs)

	for targetCurrency, pairs := range allPairs {
		var usdtPair *PairInfo
		var otherPairs []PairInfo

		// Find USDT pair and collect other pairs
		for _, pair := range pairs {
			if pair.BaseCurrency == "USDT" {
				usdtPair = &pair
			} else {
				// Only include major currencies for selling
				if isValidSellCurrency(pair.BaseCurrency) {
					otherPairs = append(otherPairs, pair)
				}
			}
		}

		// Only include if we have USDT pair AND at least one other pair
		if usdtPair != nil && len(otherPairs) > 0 {
			usdtArbitragePairs[targetCurrency] = USDTArbitragePairs{
				TargetCurrency: targetCurrency,
				USDTPair:       *usdtPair,
				OtherPairs:     otherPairs,
				LastUpdated:    time.Now(),
			}
		}
	}

	return usdtArbitragePairs
}

// isValidSellCurrency checks if the currency is suitable for selling in arbitrage
func isValidSellCurrency(currency string) bool {
	validCurrencies := map[string]bool{
		"INR":  true,
		"BTC":  true,
		"ETH":  true,
		"BNB":  true,
		"BUSD": true,
		"USDC": true,
		// Add more as needed
	}
	return validCurrencies[currency]
}

func saveUSDTArbitragePairs(pairs map[string]USDTArbitragePairs, filename string) error {
	data, err := json.MarshalIndent(pairs, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling data: %v", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	fmt.Printf("💾 Saved USDT arbitrage pairs to %s\n", filename)
	return nil
}

func displayUSDTArbitrageOpportunities(pairs map[string]USDTArbitragePairs) {
	fmt.Println("\n🎯 USDT-Based Arbitrage Opportunities:")
	fmt.Println("======================================")
	fmt.Println("💡 Strategy: Buy with USDT → Sell for other currencies")

	if len(pairs) == 0 {
		fmt.Println("❌ No USDT-based arbitrage opportunities found")
		return
	}

	opportunities := 0
	for currency, data := range pairs {
		opportunities++
		fmt.Printf("\n💰 %s (%d sell options):\n", currency, len(data.OtherPairs))
		fmt.Printf("   🟢 BUY:  %s (USDT pair)\n", data.USDTPair.Symbol)
		fmt.Printf("   🔴 SELL OPTIONS:\n")

		for _, pair := range data.OtherPairs {
			fmt.Printf("      📊 %s (%s) - Min: %.8f, Notional: %.8f\n",
				pair.Symbol, pair.BaseCurrency, pair.MinQuantity, pair.MinNotional)
		}
	}

	fmt.Printf("\n✅ Found %d currencies with USDT arbitrage potential!\n", opportunities)
	fmt.Printf("📈 Total USDT-based opportunities: %d\n", len(pairs))

	// Show summary by sell currency
	sellCurrencyCount := make(map[string]int)
	for _, data := range pairs {
		for _, pair := range data.OtherPairs {
			sellCurrencyCount[pair.BaseCurrency]++
		}
	}

	fmt.Printf("\n📊 Sell Currency Distribution:\n")
	for currency, count := range sellCurrencyCount {
		fmt.Printf("   %s: %d opportunities\n", currency, count)
	}
}
