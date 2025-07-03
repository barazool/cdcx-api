package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

// PairInfo stores essential pair information for arbitrage
type PairInfo struct {
	Symbol         string  `json:"symbol"`
	Pair           string  `json:"pair"`
	BaseCurrency   string  `json:"base_currency"`
	TargetCurrency string  `json:"target_currency"`
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

// Structures for orderbook and rates
type OrderBookResponse struct {
	Bids map[string]string `json:"bids"`
	Asks map[string]string `json:"asks"`
}

type ExchangeRate struct {
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Rate         float64   `json:"rate"`
	Timestamp    time.Time `json:"timestamp"`
	Source       string    `json:"source"`
}

type ExchangeRateCache struct {
	Rates       map[string]ExchangeRate `json:"rates"`
	LastUpdated time.Time               `json:"last_updated"`
}

type MarketLiquidity struct {
	Symbol       string  `json:"symbol"`
	Pair         string  `json:"pair"`
	BestBid      float64 `json:"best_bid"`
	BestAsk      float64 `json:"best_ask"`
	BidVolume    float64 `json:"bid_volume"`
	AskVolume    float64 `json:"ask_volume"`
	Spread       float64 `json:"spread"`
	SpreadPct    float64 `json:"spread_pct"`
	HasLiquidity bool    `json:"has_liquidity"`
}

type USDTArbitrageOpportunity struct {
	TargetCurrency  string          `json:"target_currency"`
	BuyMarketUSDT   MarketLiquidity `json:"buy_market_usdt"`   // Always USDT pair
	SellMarketOther MarketLiquidity `json:"sell_market_other"` // Other currency pair
	BuyPriceUSDT    float64         `json:"buy_price_usdt"`    // Price in USDT
	SellPriceOther  float64         `json:"sell_price_other"`  // Price in other currency
	BuyPriceINR     float64         `json:"buy_price_inr"`     // USDT price converted to INR
	SellPriceINR    float64         `json:"sell_price_inr"`    // Other currency price converted to INR
	SellCurrency    string          `json:"sell_currency"`     // Currency we're selling to (BTC, ETH, etc.)
	GrossMargin     float64         `json:"gross_margin"`      // Gross margin in INR
	GrossMarginPct  float64         `json:"gross_margin_pct"`  // Gross margin percentage
	EstimatedFees   float64         `json:"estimated_fees"`    // Estimated fees in INR
	NetMargin       float64         `json:"net_margin"`        // Net margin in INR
	NetMarginPct    float64         `json:"net_margin_pct"`    // Net margin percentage
	Viable          bool            `json:"viable"`            // Is this opportunity viable?
	TradeFlow       string          `json:"trade_flow"`        // Description of trade flow
	Timestamp       time.Time       `json:"timestamp"`
}

const (
	RATE_CACHE_FILE = "exchange_rates.json"
	CACHE_DURATION  = 5 * time.Minute
	MIN_LIQUIDITY   = 100.0 // Minimum INR value for liquidity check
	MIN_NET_MARGIN  = 2.0   // Minimum 2% net margin
)

func main() {
	fmt.Println("🚀 CoinDCX USDT-Based Arbitrage Detector")
	fmt.Println("========================================")
	fmt.Println("💡 Strategy: USDT → Buy Coin → Sell for Other Currency → Profit in INR")

	// Load USDT arbitrage pairs
	pairs, err := loadUSDTArbitragePairs("usdt_arbitrage_pairs.json")
	if err != nil {
		fmt.Printf("❌ Error loading pairs: %v\n", err)
		fmt.Println("💡 Run the USDT pair fetcher first to generate usdt_arbitrage_pairs.json")
		return
	}

	// Load exchange rate cache
	rateCache := loadExchangeRateCache()

	// Find viable USDT arbitrage opportunities
	opportunities := []USDTArbitrageOpportunity{}
	totalCurrencies := 0
	checkedCurrencies := 0

	fmt.Printf("📊 Analyzing %d currencies for USDT-based arbitrage opportunities...\n", len(pairs))

	for currency, data := range pairs {
		totalCurrencies++
		fmt.Printf("\n🔍 Analyzing %s (USDT → %d other currencies)...\n", currency, len(data.OtherPairs))

		// Get liquidity for USDT pair (buy side)
		usdtLiquidity, err := getMarketLiquidity(data.USDTPair)
		if err != nil {
			fmt.Printf("   ⚠️  USDT pair %s: %v\n", data.USDTPair.Symbol, err)
			continue
		}

		// Check USDT pair liquidity
		usdtPriceINR, err := convertToINR(usdtLiquidity.BestAsk, "USDT", &rateCache)
		if err != nil {
			fmt.Printf("   ⚠️  %s: Error converting USDT to INR: %v\n", data.USDTPair.Symbol, err)
			continue
		}

		usdtLiquidityValueINR := usdtLiquidity.AskVolume * usdtPriceINR
		if usdtLiquidityValueINR < MIN_LIQUIDITY {
			fmt.Printf("   📉 %s: Low USDT liquidity (₹%.2f)\n", data.USDTPair.Symbol, usdtLiquidityValueINR)
			continue
		}

		fmt.Printf("   ✅ USDT BUY: %s at ₹%.4f (liquidity: ₹%.2f)\n",
			data.USDTPair.Symbol, usdtPriceINR, usdtLiquidityValueINR)

		hasViableOpportunity := false

		// Check each sell option
		for _, sellPair := range data.OtherPairs {
			sellLiquidity, err := getMarketLiquidity(sellPair)
			if err != nil {
				fmt.Printf("      ⚠️  %s: %v\n", sellPair.Symbol, err)
				continue
			}

			// Check sell pair liquidity
			sellPriceINR, err := convertToINR(sellLiquidity.BestBid, sellPair.BaseCurrency, &rateCache)
			if err != nil {
				fmt.Printf("      ⚠️  %s: Error converting %s to INR: %v\n", sellPair.Symbol, sellPair.BaseCurrency, err)
				continue
			}

			sellLiquidityValueINR := sellLiquidity.BidVolume * sellPriceINR
			if sellLiquidityValueINR < MIN_LIQUIDITY {
				fmt.Printf("      📉 %s: Low %s liquidity (₹%.2f)\n", sellPair.Symbol, sellPair.BaseCurrency, sellLiquidityValueINR)
				continue
			}

			// Calculate arbitrage opportunity
			opportunity := calculateUSDTArbitrage(currency, usdtLiquidity, sellLiquidity, data, sellPair, &rateCache)
			if opportunity.Viable {
				opportunities = append(opportunities, opportunity)
				hasViableOpportunity = true
				fmt.Printf("      🎯 VIABLE: %s → %s (%.2f%% net margin)\n",
					opportunity.BuyMarketUSDT.Symbol, opportunity.SellMarketOther.Symbol, opportunity.NetMarginPct)
			} else {
				fmt.Printf("      ❌ %s → %s: %.2f%% margin (below %.1f%% threshold)\n",
					usdtLiquidity.Symbol, sellLiquidity.Symbol, opportunity.NetMarginPct, MIN_NET_MARGIN)
			}
		}

		if hasViableOpportunity {
			checkedCurrencies++
		}
	}

	// Save updated exchange rate cache
	saveExchangeRateCache(rateCache)

	// Display results
	displayUSDTResults(opportunities, totalCurrencies, checkedCurrencies)

	// Save opportunities to file
	saveUSDTOpportunities(opportunities, "usdt_arbitrage_opportunities.json")
}

func loadUSDTArbitragePairs(filename string) (map[string]USDTArbitragePairs, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var pairs map[string]USDTArbitragePairs
	err = json.Unmarshal(data, &pairs)
	return pairs, err
}

func loadExchangeRateCache() ExchangeRateCache {
	cache := ExchangeRateCache{
		Rates:       make(map[string]ExchangeRate),
		LastUpdated: time.Now(),
	}

	data, err := os.ReadFile(RATE_CACHE_FILE)
	if err != nil {
		fmt.Printf("💾 Creating new exchange rate cache\n")
		return cache
	}

	json.Unmarshal(data, &cache)
	fmt.Printf("💾 Loaded exchange rate cache with %d rates\n", len(cache.Rates))
	return cache
}

func saveExchangeRateCache(cache ExchangeRateCache) {
	cache.LastUpdated = time.Now()
	data, _ := json.MarshalIndent(cache, "", "  ")
	os.WriteFile(RATE_CACHE_FILE, data, 0644)
}

func getMarketLiquidity(pair PairInfo) (MarketLiquidity, error) {
	url := fmt.Sprintf("https://public.coindcx.com/market_data/orderbook?pair=%s", pair.Pair)

	resp, err := http.Get(url)
	if err != nil {
		return MarketLiquidity{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return MarketLiquidity{}, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MarketLiquidity{}, err
	}

	var orderbook OrderBookResponse
	if err := json.Unmarshal(body, &orderbook); err != nil {
		return MarketLiquidity{}, err
	}

	liquidity := MarketLiquidity{
		Symbol: pair.Symbol,
		Pair:   pair.Pair,
	}

	// Get best bid (highest buy price)
	if len(orderbook.Bids) > 0 {
		for priceStr, volumeStr := range orderbook.Bids {
			price, _ := strconv.ParseFloat(priceStr, 64)
			volume, _ := strconv.ParseFloat(volumeStr, 64)
			if price > liquidity.BestBid {
				liquidity.BestBid = price
				liquidity.BidVolume = volume
			}
		}
	}

	// Get best ask (lowest sell price)
	liquidity.BestAsk = 999999999.0 // Initialize with high value
	if len(orderbook.Asks) > 0 {
		for priceStr, volumeStr := range orderbook.Asks {
			price, _ := strconv.ParseFloat(priceStr, 64)
			volume, _ := strconv.ParseFloat(volumeStr, 64)
			if price < liquidity.BestAsk {
				liquidity.BestAsk = price
				liquidity.AskVolume = volume
			}
		}
	}

	// Calculate spread
	if liquidity.BestBid > 0 && liquidity.BestAsk < 999999999.0 {
		liquidity.Spread = liquidity.BestAsk - liquidity.BestBid
		liquidity.SpreadPct = (liquidity.Spread / liquidity.BestAsk) * 100
		liquidity.HasLiquidity = true
	}

	return liquidity, nil
}

func convertToINR(price float64, fromCurrency string, cache *ExchangeRateCache) (float64, error) {
	if fromCurrency == "INR" {
		return price, nil
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s_INR", fromCurrency)
	if rate, exists := cache.Rates[cacheKey]; exists {
		if time.Since(rate.Timestamp) < CACHE_DURATION {
			return price * rate.Rate, nil
		}
	}

	// Fetch new rate
	rate, err := fetchExchangeRate(fromCurrency, "INR")
	if err != nil {
		return 0, err
	}

	// Update cache
	cache.Rates[cacheKey] = rate
	return price * rate.Rate, nil
}

func fetchExchangeRate(fromCurrency, toCurrency string) (ExchangeRate, error) {
	pair := fmt.Sprintf("%s%s", fromCurrency, toCurrency)
	url := "https://api.coindcx.com/exchange/ticker"

	resp, err := http.Get(url)
	if err != nil {
		return ExchangeRate{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExchangeRate{}, err
	}

	var tickers []map[string]interface{}
	if err := json.Unmarshal(body, &tickers); err != nil {
		return ExchangeRate{}, err
	}

	for _, ticker := range tickers {
		if market, ok := ticker["market"].(string); ok && market == pair {
			if lastPriceStr, ok := ticker["last_price"].(string); ok {
				rate, err := strconv.ParseFloat(lastPriceStr, 64)
				if err == nil {
					return ExchangeRate{
						FromCurrency: fromCurrency,
						ToCurrency:   toCurrency,
						Rate:         rate,
						Timestamp:    time.Now(),
						Source:       "ticker",
					}, nil
				}
			}
		}
	}

	return ExchangeRate{}, fmt.Errorf("exchange rate not found for %s/%s", fromCurrency, toCurrency)
}

func calculateUSDTArbitrage(currency string, usdtLiquidity, sellLiquidity MarketLiquidity,
	data USDTArbitragePairs, sellPair PairInfo, cache *ExchangeRateCache) USDTArbitrageOpportunity {

	// Convert prices to INR for comparison
	buyPriceINR, err := convertToINR(usdtLiquidity.BestAsk, "USDT", cache)
	if err != nil {
		return USDTArbitrageOpportunity{}
	}

	sellPriceINR, err := convertToINR(sellLiquidity.BestBid, sellPair.BaseCurrency, cache)
	if err != nil {
		return USDTArbitrageOpportunity{}
	}

	// Calculate margins in INR terms
	grossMargin := sellPriceINR - buyPriceINR
	grossMarginPct := (grossMargin / buyPriceINR) * 100

	// Estimate fees (2% for both buy and sell transactions)
	estimatedFees := (buyPriceINR + sellPriceINR) * 0.02

	// Calculate net margins
	netMargin := grossMargin - estimatedFees
	netMarginPct := (netMargin / buyPriceINR) * 100

	tradeFlow := fmt.Sprintf("USDT → Buy %s → Sell to %s → Profit", currency, sellPair.BaseCurrency)

	return USDTArbitrageOpportunity{
		TargetCurrency:  currency,
		BuyMarketUSDT:   usdtLiquidity,
		SellMarketOther: sellLiquidity,
		BuyPriceUSDT:    usdtLiquidity.BestAsk,
		SellPriceOther:  sellLiquidity.BestBid,
		BuyPriceINR:     buyPriceINR,
		SellPriceINR:    sellPriceINR,
		SellCurrency:    sellPair.BaseCurrency,
		GrossMargin:     grossMargin,
		GrossMarginPct:  grossMarginPct,
		EstimatedFees:   estimatedFees,
		NetMargin:       netMargin,
		NetMarginPct:    netMarginPct,
		Viable:          netMarginPct >= MIN_NET_MARGIN,
		TradeFlow:       tradeFlow,
		Timestamp:       time.Now(),
	}
}

func displayUSDTResults(opportunities []USDTArbitrageOpportunity, totalCurrencies, checkedCurrencies int) {
	fmt.Printf("\n🎯 USDT-BASED ARBITRAGE ANALYSIS RESULTS\n")
	fmt.Printf("=======================================\n")
	fmt.Printf("📊 Total currencies analyzed: %d\n", totalCurrencies)
	fmt.Printf("✅ Currencies with viable opportunities: %d\n", checkedCurrencies)
	fmt.Printf("💰 Total viable arbitrage opportunities: %d\n", len(opportunities))

	if len(opportunities) == 0 {
		fmt.Printf("\n❌ No viable USDT arbitrage opportunities found with 2%+ net margin\n")
		return
	}

	// Sort opportunities by net margin percentage (highest first)
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].NetMarginPct > opportunities[j].NetMarginPct
	})

	fmt.Printf("\n🔥 VIABLE USDT ARBITRAGE OPPORTUNITIES:\n")
	fmt.Printf("=====================================\n")

	// Group by target currency for better display
	currencyOpps := make(map[string][]USDTArbitrageOpportunity)
	for _, opp := range opportunities {
		if opp.Viable {
			currencyOpps[opp.TargetCurrency] = append(currencyOpps[opp.TargetCurrency], opp)
		}
	}

	oppNum := 1
	for currency, opps := range currencyOpps {
		fmt.Printf("\n💎 %s (%d opportunities):\n", currency, len(opps))

		// Sort this currency's opportunities by margin
		sort.Slice(opps, func(i, j int) bool {
			return opps[i].NetMarginPct > opps[j].NetMarginPct
		})

		for _, opp := range opps {
			fmt.Printf("   %d. %s\n", oppNum, opp.TradeFlow)
			fmt.Printf("      🟢 BUY:  %s at ₹%.4f (USDT: %.6f)\n",
				opp.BuyMarketUSDT.Symbol, opp.BuyPriceINR, opp.BuyPriceUSDT)
			fmt.Printf("      🔴 SELL: %s at ₹%.4f (%s: %.6f)\n",
				opp.SellMarketOther.Symbol, opp.SellPriceINR, opp.SellCurrency, opp.SellPriceOther)
			fmt.Printf("      💵 Gross Margin: ₹%.4f (%.2f%%)\n", opp.GrossMargin, opp.GrossMarginPct)
			fmt.Printf("      💸 Est. Fees: ₹%.4f (2%% buffer)\n", opp.EstimatedFees)
			fmt.Printf("      💰 Net Margin: ₹%.4f (%.2f%%)\n", opp.NetMargin, opp.NetMarginPct)
			fmt.Printf("      📊 Rating: %s\n", getRatingEmoji(opp.NetMarginPct))
			oppNum++
		}
	}

	// Display summary statistics
	fmt.Printf("\n📈 SUMMARY STATISTICS:\n")
	fmt.Printf("=====================\n")

	totalOpportunities := len(opportunities)
	bestMargin := opportunities[0].NetMarginPct
	avgMargin := 0.0
	for _, opp := range opportunities {
		avgMargin += opp.NetMarginPct
	}
	avgMargin /= float64(totalOpportunities)

	fmt.Printf("📊 Best Opportunity: %.2f%% net margin (%s)\n", bestMargin, opportunities[0].TargetCurrency)
	fmt.Printf("📊 Average Margin: %.2f%%\n", avgMargin)

	// Count by sell currency
	sellCurrencyCount := make(map[string]int)
	for _, opp := range opportunities {
		sellCurrencyCount[opp.SellCurrency]++
	}

	fmt.Printf("📊 Opportunities by Sell Currency:\n")
	for currency, count := range sellCurrencyCount {
		fmt.Printf("   %s: %d opportunities\n", currency, count)
	}
}

func getRatingEmoji(netMarginPct float64) string {
	if netMarginPct >= 5.0 {
		return "🔥 EXCELLENT"
	} else if netMarginPct >= 3.5 {
		return "⭐ VERY GOOD"
	} else if netMarginPct >= 2.5 {
		return "✅ GOOD"
	} else {
		return "⚠️  MARGINAL"
	}
}

func saveUSDTOpportunities(opportunities []USDTArbitrageOpportunity, filename string) error {
	data, err := json.MarshalIndent(opportunities, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("\n💾 Saved %d USDT arbitrage opportunities to %s\n", len(opportunities), filename)
	return nil
}
