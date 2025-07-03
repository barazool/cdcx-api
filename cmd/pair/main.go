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

// ArbitragePairs stores pairs for the same target currency
type ArbitragePairs struct {
	TargetCurrency string     `json:"target_currency"`
	Pairs          []PairInfo `json:"pairs"`
	LastUpdated    time.Time  `json:"last_updated"`
}

func main() {
	fmt.Println("🔍 CoinDCX Market Data Fetcher")
	fmt.Println("================================")

	// Fetch market details
	fmt.Println("\n📊 Fetching market details...")
	markets, err := fetchMarketDetails()
	if err != nil {
		fmt.Printf("❌ Error fetching markets: %v\n", err)
		return
	}

	fmt.Printf("✅ Found %d total markets\n", len(markets))

	// Extract and group pairs by target currency
	arbitragePairs := groupPairsByTargetCurrency(markets)

	// Save to file
	err = saveArbitragePairs(arbitragePairs, "arbitrage_pairs.json")
	if err != nil {
		fmt.Printf("❌ Error saving pairs: %v\n", err)
		return
	}

	// Display interesting pairs (currencies with multiple base pairs)
	displayArbitrageOpportunities(arbitragePairs)
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

func groupPairsByTargetCurrency(markets []MarketDetail) map[string]ArbitragePairs {
	grouped := make(map[string]ArbitragePairs)

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

		if existing, exists := grouped[targetCurrency]; exists {
			existing.Pairs = append(existing.Pairs, pairInfo)
			existing.LastUpdated = time.Now()
			grouped[targetCurrency] = existing
		} else {
			grouped[targetCurrency] = ArbitragePairs{
				TargetCurrency: targetCurrency,
				Pairs:          []PairInfo{pairInfo},
				LastUpdated:    time.Now(),
			}
		}
	}

	return grouped
}

func saveArbitragePairs(pairs map[string]ArbitragePairs, filename string) error {
	data, err := json.MarshalIndent(pairs, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling data: %v", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	fmt.Printf("💾 Saved arbitrage pairs to %s\n", filename)
	return nil
}

func displayArbitrageOpportunities(pairs map[string]ArbitragePairs) {
	fmt.Println("\n🎯 Potential Arbitrage Opportunities:")
	fmt.Println("=====================================")

	// Find currencies with multiple trading pairs
	opportunities := 0
	for currency, data := range pairs {
		if len(data.Pairs) > 1 {
			opportunities++
			fmt.Printf("\n💰 %s (%d pairs):\n", currency, len(data.Pairs))

			for _, pair := range data.Pairs {
				fmt.Printf("   📊 %s (%s) - Min: %.8f, Notional: %.8f\n",
					pair.Symbol, pair.BaseCurrency, pair.MinQuantity, pair.MinNotional)
			}
		}
	}

	if opportunities == 0 {
		fmt.Println("❌ No arbitrage opportunities found (no currencies with multiple base pairs)")
	} else {
		fmt.Printf("\n✅ Found %d currencies with arbitrage potential!\n", opportunities)

		// Special highlight for RENDER
		if renderData, exists := pairs["RENDER"]; exists {
			fmt.Printf("\n🔥 RENDER Analysis:\n")
			for _, pair := range renderData.Pairs {
				fmt.Printf("   %s: Min Qty: %.8f, Min Notional: %.8f\n",
					pair.Symbol, pair.MinQuantity, pair.MinNotional)
			}
		}
	}

	fmt.Printf("\n📈 Total currencies tracked: %d\n", len(pairs))
}
