package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

// ArbitrageOpportunity structure to match saved data
type ArbitrageOpportunity struct {
	TargetCurrency string `json:"target_currency"`
	BuyMarket      struct {
		Symbol string `json:"symbol"`
		Pair   string `json:"pair"`
	} `json:"buy_market"`
	SellMarket struct {
		Symbol string `json:"symbol"`
		Pair   string `json:"pair"`
	} `json:"sell_market"`
	BuyPriceINR    float64   `json:"buy_price_inr"`
	SellPriceINR   float64   `json:"sell_price_inr"`
	GrossMargin    float64   `json:"gross_margin"`
	GrossMarginPct float64   `json:"gross_margin_pct"`
	EstimatedFees  float64   `json:"estimated_fees"`
	NetMargin      float64   `json:"net_margin"`
	NetMarginPct   float64   `json:"net_margin_pct"`
	Viable         bool      `json:"viable"`
	Timestamp      time.Time `json:"timestamp"`
}

// Existing structures
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

// Enhanced order book structures
type OrderBookLevel struct {
	Price      float64 `json:"price"`
	Volume     float64 `json:"volume"`
	PriceINR   float64 `json:"price_inr"`
	Cumulative float64 `json:"cumulative"`
	VolumeINR  float64 `json:"volume_inr"`
}

type EnhancedOrderBook struct {
	Symbol         string           `json:"symbol"`
	Pair           string           `json:"pair"`
	BaseCurrency   string           `json:"base_currency"`
	BidLevels      []OrderBookLevel `json:"bid_levels"`
	AskLevels      []OrderBookLevel `json:"ask_levels"`
	BestBid        float64          `json:"best_bid"`
	BestAsk        float64          `json:"best_ask"`
	BestBidINR     float64          `json:"best_bid_inr"`
	BestAskINR     float64          `json:"best_ask_inr"`
	Spread         float64          `json:"spread"`
	SpreadPct      float64          `json:"spread_pct"`
	TotalBidVolume float64          `json:"total_bid_volume"`
	TotalAskVolume float64          `json:"total_ask_volume"`
	Timestamp      time.Time        `json:"timestamp"`
}

type OrderSimulation struct {
	OrderNumber    int     `json:"order_number"`
	BuyPrice       float64 `json:"buy_price"`
	SellPrice      float64 `json:"sell_price"`
	Volume         float64 `json:"volume"`
	VolumeINR      float64 `json:"volume_inr"`
	GrossMargin    float64 `json:"gross_margin"`
	GrossMarginPct float64 `json:"gross_margin_pct"`
	EstimatedFees  float64 `json:"estimated_fees"`
	NetMargin      float64 `json:"net_margin"`
	NetMarginPct   float64 `json:"net_margin_pct"`
	Profitable     bool    `json:"profitable"`
	Cumulative     struct {
		Volume    float64 `json:"volume"`
		VolumeINR float64 `json:"volume_inr"`
		NetProfit float64 `json:"net_profit"`
	} `json:"cumulative"`
}

type ArbitrageDepthAnalysis struct {
	Currency              string            `json:"currency"`
	BuyMarket             EnhancedOrderBook `json:"buy_market"`
	SellMarket            EnhancedOrderBook `json:"sell_market"`
	OrderSimulations      []OrderSimulation `json:"order_simulations"`
	MaxProfitableOrders   int               `json:"max_profitable_orders"`
	TotalProfitableVolume float64           `json:"total_profitable_volume"`
	TotalEstimatedProfit  float64           `json:"total_estimated_profit"`
	BottleneckSide        string            `json:"bottleneck_side"`
	OpportunityRating     string            `json:"opportunity_rating"`
	Timestamp             time.Time         `json:"timestamp"`
}

const (
	RATE_CACHE_FILE = "exchange_rates.json"
	CACHE_DURATION  = 5 * time.Minute
	MIN_NET_MARGIN  = 2.0
	MAX_LEVELS      = 10
	FEE_RATE        = 0.02
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🔬 CoinDCX Order Book Depth Analyzer")
	fmt.Println("=====================================")
	fmt.Println("⚠️  ANALYSIS MODE - NO EXECUTION")
	fmt.Println("🔍 Deep diving into arbitrage opportunities...")

	// Load viable opportunities from previous analysis
	viableOpportunities, err := loadViableOpportunities("arbitrage_opportunities.json")
	if err != nil {
		log.Fatalf("❌ Error loading opportunities: %v\nRun arbitrage detector first!", err)
	}

	// Extract unique currencies from viable opportunities
	targetCurrencies := extractUniqueCurrencies(viableOpportunities)

	fmt.Printf("\n🎯 Found %d viable opportunities covering %d currencies\n", len(viableOpportunities), len(targetCurrencies))
	fmt.Printf("📋 Currencies to analyze: %v\n", targetCurrencies)

	// Load saved pairs
	pairs, err := loadArbitragePairs("arbitrage_pairs.json")
	if err != nil {
		log.Fatalf("❌ Error loading pairs: %v", err)
	}

	// Load exchange rate cache
	rateCache := loadExchangeRateCache()

	fmt.Printf("\n🎯 Analyzing %d currencies for order book depth...\n", len(targetCurrencies))

	allAnalyses := []ArbitrageDepthAnalysis{}

	for _, currency := range targetCurrencies {
		if data, exists := pairs[currency]; exists {
			if len(data.Pairs) < 2 {
				log.Printf("⚠️ %s: Insufficient pairs for arbitrage", currency)
				continue
			}

			log.Printf("\n🔍 ANALYZING %s (%d pairs)...", currency, len(data.Pairs))

			analysis, err := analyzeArbitrageDepth(currency, data, &rateCache)
			if err != nil {
				log.Printf("❌ %s: Analysis failed: %v", currency, err)
				continue
			}

			allAnalyses = append(allAnalyses, analysis...)
		} else {
			log.Printf("⚠️ %s: Not found in pairs data", currency)
		}
	}

	// Save updated cache
	saveExchangeRateCache(rateCache)

	// Display detailed results
	displayDepthAnalysis(allAnalyses)

	// Save detailed analysis
	saveDepthAnalysis(allAnalyses, "depth_analysis.json")
}

func loadViableOpportunities(filename string) ([]ArbitrageOpportunity, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", filename, err)
	}

	var opportunities []ArbitrageOpportunity
	err = json.Unmarshal(data, &opportunities)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %v", filename, err)
	}

	// Filter only viable opportunities
	viable := []ArbitrageOpportunity{}
	for _, opp := range opportunities {
		if opp.Viable {
			viable = append(viable, opp)
		}
	}

	log.Printf("💾 Loaded %d viable opportunities from %s", len(viable), filename)
	return viable, nil
}

func extractUniqueCurrencies(opportunities []ArbitrageOpportunity) []string {
	currencyMap := make(map[string]bool)

	for _, opp := range opportunities {
		currencyMap[opp.TargetCurrency] = true
	}

	currencies := []string{}
	for currency := range currencyMap {
		currencies = append(currencies, currency)
	}

	return currencies
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

func analyzeArbitrageDepth(currency string, data ArbitragePairs, cache *ExchangeRateCache) ([]ArbitrageDepthAnalysis, error) {
	log.Printf("📊 Fetching order books for %s pairs...", currency)

	orderBooks := []EnhancedOrderBook{}

	for _, pair := range data.Pairs {
		log.Printf("   📖 Fetching %s order book...", pair.Symbol)

		orderBook, err := getEnhancedOrderBook(pair, cache)
		if err != nil {
			log.Printf("   ❌ %s: %v", pair.Symbol, err)
			continue
		}

		log.Printf("   ✅ %s: Best Ask ₹%.4f (%d levels), Best Bid ₹%.4f (%d levels)",
			pair.Symbol, orderBook.BestAskINR, len(orderBook.AskLevels),
			orderBook.BestBidINR, len(orderBook.BidLevels))

		orderBooks = append(orderBooks, orderBook)
	}

	if len(orderBooks) < 2 {
		return nil, fmt.Errorf("insufficient order book data")
	}

	analyses := []ArbitrageDepthAnalysis{}

	for i := 0; i < len(orderBooks); i++ {
		for j := i + 1; j < len(orderBooks); j++ {
			analysis1 := simulateArbitrageDepth(currency, orderBooks[i], orderBooks[j])
			analysis2 := simulateArbitrageDepth(currency, orderBooks[j], orderBooks[i])

			if analysis1.MaxProfitableOrders > 0 {
				analyses = append(analyses, analysis1)
			}
			if analysis2.MaxProfitableOrders > 0 {
				analyses = append(analyses, analysis2)
			}
		}
	}

	return analyses, nil
}

func getEnhancedOrderBook(pair PairInfo, cache *ExchangeRateCache) (EnhancedOrderBook, error) {
	url := fmt.Sprintf("https://public.coindcx.com/market_data/orderbook?pair=%s", pair.Pair)

	log.Printf("   🌐 API Request: %s", url)

	// client := &http.Client{Timeout: 15 * time.Second}
	resp, err := http.Get(url)
	if err != nil {
		return EnhancedOrderBook{}, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return EnhancedOrderBook{}, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return EnhancedOrderBook{}, fmt.Errorf("read error: %v", err)
	}

	var rawOrderBook map[string]interface{}
	if err := json.Unmarshal(body, &rawOrderBook); err != nil {
		return EnhancedOrderBook{}, fmt.Errorf("parse error: %v", err)
	}

	log.Printf("   📊 Raw order book received, processing levels...")

	enhanced := EnhancedOrderBook{
		Symbol:       pair.Symbol,
		Pair:         pair.Pair,
		BaseCurrency: pair.BaseCurrency,
		Timestamp:    time.Now(),
	}

	if bids, ok := rawOrderBook["bids"].(map[string]interface{}); ok {
		enhanced.BidLevels = processOrderBookSide(bids, pair.BaseCurrency, cache, "bid")
		if len(enhanced.BidLevels) > 0 {
			enhanced.BestBid = enhanced.BidLevels[0].Price
			enhanced.BestBidINR = enhanced.BidLevels[0].PriceINR
		}
	}

	if asks, ok := rawOrderBook["asks"].(map[string]interface{}); ok {
		enhanced.AskLevels = processOrderBookSide(asks, pair.BaseCurrency, cache, "ask")
		if len(enhanced.AskLevels) > 0 {
			enhanced.BestAsk = enhanced.AskLevels[0].Price
			enhanced.BestAskINR = enhanced.AskLevels[0].PriceINR
		}
	}

	if enhanced.BestBid > 0 && enhanced.BestAsk > 0 {
		enhanced.Spread = enhanced.BestAsk - enhanced.BestBid
		enhanced.SpreadPct = (enhanced.Spread / enhanced.BestAsk) * 100
	}

	for _, level := range enhanced.BidLevels {
		enhanced.TotalBidVolume += level.Volume
	}
	for _, level := range enhanced.AskLevels {
		enhanced.TotalAskVolume += level.Volume
	}

	log.Printf("   📈 Processed: %d bid levels, %d ask levels, spread: %.2f%%",
		len(enhanced.BidLevels), len(enhanced.AskLevels), enhanced.SpreadPct)

	return enhanced, nil
}

func processOrderBookSide(orders map[string]interface{}, baseCurrency string, cache *ExchangeRateCache, side string) []OrderBookLevel {
	type priceLevel struct {
		price  float64
		volume float64
	}

	levels := []priceLevel{}

	for priceStr, volumeInterface := range orders {
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			continue
		}

		var volume float64
		switch v := volumeInterface.(type) {
		case string:
			volume, _ = strconv.ParseFloat(v, 64)
		case float64:
			volume = v
		}

		if volume > 0 {
			levels = append(levels, priceLevel{price: price, volume: volume})
		}
	}

	if side == "bid" {
		sort.Slice(levels, func(i, j int) bool {
			return levels[i].price > levels[j].price
		})
	} else {
		sort.Slice(levels, func(i, j int) bool {
			return levels[i].price < levels[j].price
		})
	}

	enhanced := []OrderBookLevel{}
	cumulative := 0.0

	maxLevels := MAX_LEVELS
	if len(levels) < maxLevels {
		maxLevels = len(levels)
	}

	for i := 0; i < maxLevels; i++ {
		level := levels[i]

		priceINR, err := convertToINR(level.price, baseCurrency, cache)
		if err != nil {
			log.Printf("      ⚠️ Price conversion failed for %f %s: %v", level.price, baseCurrency, err)
			continue
		}

		cumulative += level.volume

		enhanced = append(enhanced, OrderBookLevel{
			Price:      level.price,
			Volume:     level.volume,
			PriceINR:   priceINR,
			Cumulative: cumulative,
			VolumeINR:  level.volume * priceINR,
		})

		log.Printf("      📊 Level %d: %f %s (₹%.4f) - Vol: %.4f (₹%.2f)",
			i+1, level.price, baseCurrency, priceINR, level.volume, level.volume*priceINR)
	}

	return enhanced
}

func simulateArbitrageDepth(currency string, buyMarket, sellMarket EnhancedOrderBook) ArbitrageDepthAnalysis {
	log.Printf("\n🧮 SIMULATING ARBITRAGE: %s", currency)
	log.Printf("   🟢 BUY from %s (best: ₹%.4f)", buyMarket.Symbol, buyMarket.BestAskINR)
	log.Printf("   🔴 SELL to %s (best: ₹%.4f)", sellMarket.Symbol, sellMarket.BestBidINR)

	analysis := ArbitrageDepthAnalysis{
		Currency:   currency,
		BuyMarket:  buyMarket,
		SellMarket: sellMarket,
		Timestamp:  time.Now(),
	}

	if buyMarket.BestAskINR >= sellMarket.BestBidINR {
		log.Printf("   ❌ No arbitrage opportunity: buy ₹%.4f >= sell ₹%.4f",
			buyMarket.BestAskINR, sellMarket.BestBidINR)
		return analysis
	}

	log.Printf("   ✅ Initial arbitrage margin: ₹%.4f (%.2f%%)",
		sellMarket.BestBidINR-buyMarket.BestAskINR,
		((sellMarket.BestBidINR-buyMarket.BestAskINR)/buyMarket.BestAskINR)*100)

	// Simulate step by step order execution
	buyLevelIdx := 0
	sellLevelIdx := 0
	orderNumber := 1

	cumulativeVolume := 0.0
	cumulativeVolumeINR := 0.0
	cumulativeNetProfit := 0.0

	for buyLevelIdx < len(buyMarket.AskLevels) && sellLevelIdx < len(sellMarket.BidLevels) {
		buyLevel := buyMarket.AskLevels[buyLevelIdx]
		sellLevel := sellMarket.BidLevels[sellLevelIdx]

		// Determine tradeable volume (limited by smaller side)
		tradeableVolume := buyLevel.Volume
		if sellLevel.Volume < tradeableVolume {
			tradeableVolume = sellLevel.Volume
		}

		// Calculate prices and margins
		buyPriceINR := buyLevel.PriceINR
		sellPriceINR := sellLevel.PriceINR

		grossMargin := sellPriceINR - buyPriceINR
		grossMarginPct := (grossMargin / buyPriceINR) * 100

		// Calculate fees and net margin
		tradeValueINR := tradeableVolume * buyPriceINR
		estimatedFees := tradeValueINR * FEE_RATE
		netMargin := (grossMargin * tradeableVolume) - estimatedFees
		netMarginPct := (netMargin / tradeValueINR) * 100

		log.Printf("   📋 Order %d: Vol %.4f, Buy ₹%.4f, Sell ₹%.4f, Net Margin %.2f%%",
			orderNumber, tradeableVolume, buyPriceINR, sellPriceINR, netMarginPct)

		// Check if still profitable
		profitable := netMarginPct >= MIN_NET_MARGIN

		if profitable {
			cumulativeVolume += tradeableVolume
			cumulativeVolumeINR += tradeValueINR
			cumulativeNetProfit += netMargin

			simulation := OrderSimulation{
				OrderNumber:    orderNumber,
				BuyPrice:       buyPriceINR,
				SellPrice:      sellPriceINR,
				Volume:         tradeableVolume,
				VolumeINR:      tradeValueINR,
				GrossMargin:    grossMargin,
				GrossMarginPct: grossMarginPct,
				EstimatedFees:  estimatedFees,
				NetMargin:      netMargin,
				NetMarginPct:   netMarginPct,
				Profitable:     true,
			}
			simulation.Cumulative.Volume = cumulativeVolume
			simulation.Cumulative.VolumeINR = cumulativeVolumeINR
			simulation.Cumulative.NetProfit = cumulativeNetProfit

			analysis.OrderSimulations = append(analysis.OrderSimulations, simulation)
			analysis.MaxProfitableOrders = orderNumber

			log.Printf("      ✅ Profitable! Net: ₹%.2f, Cumulative: ₹%.2f", netMargin, cumulativeNetProfit)
		} else {
			log.Printf("      ❌ No longer profitable (%.2f%% < %.1f%%)", netMarginPct, MIN_NET_MARGIN)
			break
		}

		// Move to next levels
		if buyLevel.Volume <= sellLevel.Volume {
			buyLevelIdx++
		}
		if sellLevel.Volume <= buyLevel.Volume {
			sellLevelIdx++
		}

		orderNumber++
	}

	analysis.TotalProfitableVolume = cumulativeVolume
	analysis.TotalEstimatedProfit = cumulativeNetProfit

	// Determine bottleneck
	if buyLevelIdx >= len(buyMarket.AskLevels) {
		analysis.BottleneckSide = "buy"
	} else {
		analysis.BottleneckSide = "sell"
	}

	// Rate opportunity
	if analysis.MaxProfitableOrders >= 5 {
		analysis.OpportunityRating = "excellent"
	} else if analysis.MaxProfitableOrders >= 3 {
		analysis.OpportunityRating = "good"
	} else {
		analysis.OpportunityRating = "poor"
	}

	log.Printf("   🎯 RESULT: %d profitable orders, ₹%.2f total profit, %s rating",
		analysis.MaxProfitableOrders, analysis.TotalEstimatedProfit, analysis.OpportunityRating)

	return analysis
}

func displayDepthAnalysis(analyses []ArbitrageDepthAnalysis) {
	fmt.Printf("\n🎯 ORDER BOOK DEPTH ANALYSIS RESULTS\n")
	fmt.Printf("====================================\n")

	if len(analyses) == 0 {
		fmt.Printf("❌ No profitable arbitrage depth found\n")
		return
	}

	// Sort by total profit descending
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].TotalEstimatedProfit > analyses[j].TotalEstimatedProfit
	})

	for i, analysis := range analyses {
		fmt.Printf("\n%d. 💎 %s (%s)\n", i+1, analysis.Currency, analysis.OpportunityRating)
		fmt.Printf("   🟢 BUY:  %s → 🔴 SELL: %s\n", analysis.BuyMarket.Symbol, analysis.SellMarket.Symbol)
		fmt.Printf("   📊 Max Orders: %d | Total Volume: %.4f tokens (₹%.2f)\n",
			analysis.MaxProfitableOrders, analysis.TotalProfitableVolume,
			analyses[i].OrderSimulations[len(analyses[i].OrderSimulations)-1].Cumulative.VolumeINR)
		fmt.Printf("   💰 Total Estimated Profit: ₹%.2f\n", analysis.TotalEstimatedProfit)
		fmt.Printf("   ⚖️  Bottleneck: %s side\n", analysis.BottleneckSide)

		if len(analysis.OrderSimulations) > 0 {
			fmt.Printf("   📋 Order Breakdown:\n")
			for j, sim := range analysis.OrderSimulations {
				if j < 3 { // Show first 3 orders
					fmt.Printf("      %d. Vol: %.4f @ ₹%.4f→₹%.4f = ₹%.2f profit (%.2f%%)\n",
						sim.OrderNumber, sim.Volume, sim.BuyPrice, sim.SellPrice,
						sim.NetMargin, sim.NetMarginPct)
				}
			}
			if len(analysis.OrderSimulations) > 3 {
				fmt.Printf("      ... and %d more orders\n", len(analysis.OrderSimulations)-3)
			}
		}
	}
}

func saveDepthAnalysis(analyses []ArbitrageDepthAnalysis, filename string) error {
	data, err := json.MarshalIndent(analyses, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("\n💾 Saved detailed depth analysis to %s\n", filename)
	return nil
}
