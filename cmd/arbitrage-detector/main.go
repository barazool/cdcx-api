package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Existing structures from market fetcher
type PairInfo struct {
	Symbol         string  `json:"symbol"`
	Pair           string  `json:"pair"`
	BaseCurrency   string  `json:"base_currency"`
	TargetCurrency string  `json:"target_currency"`
	MinQuantity    float64 `json:"min_quantity"`
	MinNotional    float64 `json:"min_notional"`
	Status         string  `json:"status"`
}

type ArbitragePairs struct {
	TargetCurrency string     `json:"target_currency"`
	Pairs          []PairInfo `json:"pairs"`
	LastUpdated    time.Time  `json:"last_updated"`
}

// New structures for orderbook and rates
type OrderBookResponse struct {
	Bids map[string]string `json:"bids"`
	Asks map[string]string `json:"asks"`
}

type ExchangeRate struct {
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Rate         float64   `json:"rate"`
	Timestamp    time.Time `json:"timestamp"`
	Source       string    `json:"source"` // ticker, orderbook
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

type ArbitrageOpportunity struct {
	TargetCurrency string          `json:"target_currency"`
	BuyMarket      MarketLiquidity `json:"buy_market"`
	SellMarket     MarketLiquidity `json:"sell_market"`
	BuyPriceINR    float64         `json:"buy_price_inr"`
	SellPriceINR   float64         `json:"sell_price_inr"`
	GrossMargin    float64         `json:"gross_margin"`
	GrossMarginPct float64         `json:"gross_margin_pct"`
	EstimatedFees  float64         `json:"estimated_fees"`
	NetMargin      float64         `json:"net_margin"`
	NetMarginPct   float64         `json:"net_margin_pct"`
	Viable         bool            `json:"viable"`
	Timestamp      time.Time       `json:"timestamp"`
}

const (
	RATE_CACHE_FILE = "exchange_rates.json"
	CACHE_DURATION  = 5 * time.Minute
	MIN_LIQUIDITY   = 100.0 // Minimum INR value for liquidity check
	MIN_NET_MARGIN  = 2.0   // Minimum 2% net margin
)

func main() {
	fmt.Println("🚀 CoinDCX Arbitrage Detector")
	fmt.Println("=============================")

	// Load saved pairs
	pairs, err := loadArbitragePairs("arbitrage_pairs.json")
	if err != nil {
		fmt.Printf("❌ Error loading pairs: %v\n", err)
		fmt.Println("💡 Run the market fetcher first to generate arbitrage_pairs.json")
		return
	}

	// Load exchange rate cache
	rateCache := loadExchangeRateCache()

	// Find viable arbitrage opportunities
	opportunities := []ArbitrageOpportunity{}
	totalPairs := 0
	checkedPairs := 0

	fmt.Printf("📊 Analyzing %d currencies for arbitrage opportunities...\n", len(pairs))

	for currency, data := range pairs {
		if len(data.Pairs) < 2 {
			continue // Skip currencies with only one pair
		}

		totalPairs++
		fmt.Printf("\n🔍 Analyzing %s (%d pairs)...\n", currency, len(data.Pairs))

		// Get liquidity data for all pairs of this currency
		liquidityData := []MarketLiquidity{}
		for _, pair := range data.Pairs {
			liquidity, err := getMarketLiquidity(pair)
			if err != nil {
				fmt.Printf("   ⚠️  %s: %v\n", pair.Symbol, err)
				continue
			}

			// Convert price to INR
			priceINR, err := convertToINR(liquidity.BestAsk, pair.BaseCurrency, &rateCache)
			if err != nil {
				fmt.Printf("   ⚠️  %s: Error converting to INR: %v\n", pair.Symbol, err)
				continue
			}

			// Check if market has sufficient liquidity
			liquidityValueINR := liquidity.AskVolume * priceINR
			if liquidityValueINR < MIN_LIQUIDITY {
				fmt.Printf("   📉 %s: Low liquidity (₹%.2f)\n", pair.Symbol, liquidityValueINR)
				continue
			}

			liquidityData = append(liquidityData, liquidity)
			fmt.Printf("   ✅ %s: Bid ₹%.2f, Ask ₹%.2f, Spread %.2f%%\n",
				pair.Symbol, liquidity.BestBid, priceINR, liquidity.SpreadPct)
		}

		if len(liquidityData) < 2 {
			fmt.Printf("   ❌ %s: Insufficient liquid markets\n", currency)
			continue
		}

		checkedPairs++

		// Find arbitrage opportunities between pairs
		for i := 0; i < len(liquidityData); i++ {
			for j := i + 1; j < len(liquidityData); j++ {
				opportunity := calculateArbitrage(currency, liquidityData[i], liquidityData[j], data.Pairs, &rateCache)
				if opportunity.Viable {
					opportunities = append(opportunities, opportunity)
				}
			}
		}
	}

	// Save updated exchange rate cache
	saveExchangeRateCache(rateCache)

	// Display results
	displayResults(opportunities, totalPairs, checkedPairs)

	// Save opportunities to file
	saveOpportunities(opportunities, "arbitrage_opportunities.json")
}

func loadArbitragePairs(filename string) (map[string]ArbitragePairs, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var pairs map[string]ArbitragePairs
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

	// client := &http.Client{Timeout: 10 * time.Second}
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
	// Try to get rate from CoinDCX ticker
	pair := fmt.Sprintf("%s%s", fromCurrency, toCurrency)
	url := "https://api.coindcx.com/exchange/ticker"

	// client := &http.Client{Timeout: 10 * time.Second}
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

	// Find the ticker for our pair
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

func calculateArbitrage(currency string, market1, market2 MarketLiquidity, pairs []PairInfo, cache *ExchangeRateCache) ArbitrageOpportunity {
	// Convert both prices to INR
	var price1INR, price2INR float64
	var err error

	// Find the base currencies for each market
	var base1, base2 string
	for _, pair := range pairs {
		if pair.Symbol == market1.Symbol {
			base1 = pair.BaseCurrency
		}
		if pair.Symbol == market2.Symbol {
			base2 = pair.BaseCurrency
		}
	}

	price1INR, err = convertToINR(market1.BestAsk, base1, cache)
	if err != nil {
		return ArbitrageOpportunity{}
	}

	price2INR, err = convertToINR(market2.BestAsk, base2, cache)
	if err != nil {
		return ArbitrageOpportunity{}
	}

	// Determine buy and sell markets
	var buyMarket, sellMarket MarketLiquidity
	var buyPriceINR, sellPriceINR float64

	if price1INR < price2INR {
		buyMarket = market1
		sellMarket = market2
		buyPriceINR = price1INR
		sellPriceINR = price2INR
	} else {
		buyMarket = market2
		sellMarket = market1
		buyPriceINR = price2INR
		sellPriceINR = price1INR
	}

	// Calculate margins
	grossMargin := sellPriceINR - buyPriceINR
	grossMarginPct := (grossMargin / buyPriceINR) * 100

	// Estimate fees (using 2% conservative estimate)
	estimatedFees := (buyPriceINR + sellPriceINR) * 0.02

	// Calculate net margins
	netMargin := grossMargin - estimatedFees
	netMarginPct := (netMargin / buyPriceINR) * 100

	return ArbitrageOpportunity{
		TargetCurrency: currency,
		BuyMarket:      buyMarket,
		SellMarket:     sellMarket,
		BuyPriceINR:    buyPriceINR,
		SellPriceINR:   sellPriceINR,
		GrossMargin:    grossMargin,
		GrossMarginPct: grossMarginPct,
		EstimatedFees:  estimatedFees,
		NetMargin:      netMargin,
		NetMarginPct:   netMarginPct,
		Viable:         netMarginPct >= MIN_NET_MARGIN,
		Timestamp:      time.Now(),
	}
}

func displayResults(opportunities []ArbitrageOpportunity, totalPairs, checkedPairs int) {
	fmt.Printf("\n🎯 ARBITRAGE ANALYSIS RESULTS\n")
	fmt.Printf("============================\n")
	fmt.Printf("📊 Total currencies with multiple pairs: %d\n", totalPairs)
	fmt.Printf("✅ Currencies with sufficient liquidity: %d\n", checkedPairs)
	fmt.Printf("💰 Viable arbitrage opportunities: %d\n", len(opportunities))

	if len(opportunities) == 0 {
		fmt.Printf("\n❌ No viable arbitrage opportunities found with 2%+ net margin\n")
		return
	}

	fmt.Printf("\n🔥 VIABLE OPPORTUNITIES:\n")
	fmt.Printf("========================\n")

	for i, opp := range opportunities {
		if opp.Viable {
			fmt.Printf("\n%d. 💎 %s\n", i+1, opp.TargetCurrency)
			fmt.Printf("   🟢 BUY:  %s at ₹%.4f\n", opp.BuyMarket.Symbol, opp.BuyPriceINR)
			fmt.Printf("   🔴 SELL: %s at ₹%.4f\n", opp.SellMarket.Symbol, opp.SellPriceINR)
			fmt.Printf("   💵 Gross Margin: ₹%.4f (%.2f%%)\n", opp.GrossMargin, opp.GrossMarginPct)
			fmt.Printf("   💸 Est. Fees: ₹%.4f (2%% buffer)\n", opp.EstimatedFees)
			fmt.Printf("   💰 Net Margin: ₹%.4f (%.2f%%)\n", opp.NetMargin, opp.NetMarginPct)
		}
	}
}

func saveOpportunities(opportunities []ArbitrageOpportunity, filename string) error {
	data, err := json.MarshalIndent(opportunities, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("\n💾 Saved %d opportunities to %s\n", len(opportunities), filename)
	return nil
}
