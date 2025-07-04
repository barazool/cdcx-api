package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/arbitrage"
	"github.com/b-thark/cdcx-api/pkg/exchange"
	"github.com/b-thark/cdcx-api/pkg/market"
	"github.com/b-thark/cdcx-api/pkg/pairs"
	"github.com/b-thark/cdcx-api/pkg/types"
)

var (
	executionMutex sync.Mutex // Global execution lock
	wg             sync.WaitGroup
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🚀 CoinDCX Live Arbitrage Detector")
	fmt.Println("==================================")
	fmt.Println("⚠️  LIVE TRADING MODE - REAL EXECUTION")
	fmt.Println("🔍 Real-time detection → immediate execution")

	// Load configurations
	tradingConfig := types.DefaultConfig()
	execConfig := types.DefaultExecutionConfig()

	apiConfig, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Error loading API config: %v", err)
	}

	// Allow environment overrides
	if stopLoss := os.Getenv("STOP_LOSS_PCT"); stopLoss != "" {
		if val := parseFloat(stopLoss); val > 0 {
			execConfig.StopLossPct = val
			fmt.Printf("🛑 Custom stop loss: %.1f%%\n", val)
		}
	}

	if maxPosition := os.Getenv("MAX_POSITION_USDT"); maxPosition != "" {
		if val := parseFloat(maxPosition); val > 0 {
			execConfig.MaxPositionUSDT = val
			fmt.Printf("💰 Custom max position: $%.2f\n", val)
		}
	}

	if minMargin := os.Getenv("MIN_NET_MARGIN"); minMargin != "" {
		if margin := parseFloat(minMargin); margin > 0 {
			tradingConfig.MinNetMargin = margin
			fmt.Printf("🎯 Custom minimum net margin: %.1f%%\n", margin)
		}
	}

	// Load arbitrage pairs
	fmt.Println("\n📂 Loading arbitrage pairs...")
	pairAnalyzer := pairs.NewAnalyzer(tradingConfig)
	arbitragePairs, err := pairAnalyzer.LoadPairs("arbitrage_pairs.json")
	if err != nil {
		log.Fatalf("❌ Error loading pairs: %v\n💡 Run pair detector first: go run cmd/pair-detector/main.go", err)
	}

	fmt.Printf("✅ Loaded %d currencies with arbitrage potential\n", len(arbitragePairs))

	// Create components
	fetcher := market.NewFetcher()
	rateManager := exchange.NewRateManager(tradingConfig)
	engine := arbitrage.NewEngine(apiConfig, execConfig)

	// Check account readiness
	fmt.Println("\n🔍 Checking account status...")
	ready, err := engine.CheckAccountReadiness()
	if err != nil {
		log.Fatalf("❌ Account check failed: %v", err)
	}

	if !ready {
		fmt.Println("❌ Account not ready for execution")
		return
	}

	fmt.Println("✅ Account ready for live trading")

	// Start live detection and execution
	fmt.Println("\n🚀 Starting live arbitrage detection...")
	fmt.Println("🔒 Global execution lock: Only one trade at a time")
	fmt.Println("🔍 Detection: Parallel across all opportunities")

	totalOpportunities := 0
	for currency, pairGroup := range arbitragePairs {
		if len(pairGroup.Pairs) < 2 {
			continue
		}

		log.Printf("📊 Analyzing %s (%d pairs)...", currency, len(pairGroup.Pairs))

		// Find opportunities for this currency
		currencyOpps, err := analyzeCurrency(currency, pairGroup.Pairs, fetcher, rateManager, tradingConfig)
		if err != nil {
			log.Printf("❌ %s: %v", currency, err)
			continue
		}

		// Launch goroutine for each viable opportunity
		for _, opp := range currencyOpps {
			if opp.Viable && hasUSDTPair(opp) {
				totalOpportunities++

				log.Printf("🎯 VIABLE: %s (%s → %s) %.2f%% - LAUNCHING EXECUTION",
					opp.TargetCurrency, opp.BuyMarket.Symbol, opp.SellMarket.Symbol, opp.NetMarginPct)

				wg.Add(1)
				go executeOpportunity(engine, opp, totalOpportunities)
			}
		}
	}

	// Save rate cache
	rateManager.SaveCache()

	if totalOpportunities == 0 {
		fmt.Println("❌ No viable opportunities found")
		return
	}

	fmt.Printf("🚀 Launched %d execution goroutines\n", totalOpportunities)

	// Wait for all executions to complete
	wg.Wait()

	fmt.Println("\n🎯 All live arbitrage executions complete!")
}

// Copied and adapted from opportunity detector
func analyzeCurrency(currency string, pairs []types.PairInfo, fetcher *market.Fetcher, rateManager *exchange.RateManager, config *types.Config) ([]types.ArbitrageOpportunity, error) {
	// Get current prices for all pairs
	pairPrices := make(map[string]PriceInfo)

	for _, pair := range pairs {
		priceInfo, err := getPriceInfo(pair, fetcher, rateManager)
		if err != nil {
			log.Printf("   ⚠️ %s: %v", pair.Symbol, err)
			continue
		}

		// Check liquidity
		bidLiquidityINR := priceInfo.BidVolume * priceInfo.BestBidINR
		askLiquidityINR := priceInfo.AskVolume * priceInfo.BestAskINR

		if bidLiquidityINR < config.MinLiquidity || askLiquidityINR < config.MinLiquidity {
			log.Printf("   📉 %s: Low liquidity (₹%.2f bid, ₹%.2f ask)",
				pair.Symbol, bidLiquidityINR, askLiquidityINR)
			continue
		}

		priceInfo.HasLiquidity = true
		pairPrices[pair.Symbol] = priceInfo
	}

	if len(pairPrices) < 2 {
		return nil, fmt.Errorf("insufficient liquid pairs")
	}

	// Find arbitrage opportunities between all pair combinations
	opportunities := []types.ArbitrageOpportunity{}

	for buySymbol, buyPrice := range pairPrices {
		for sellSymbol, sellPrice := range pairPrices {
			if buySymbol == sellSymbol || !buyPrice.HasLiquidity || !sellPrice.HasLiquidity {
				continue
			}

			opp := calculateArbitrage(currency, buyPrice, sellPrice, config)
			if opp.NetMarginPct >= config.MinNetMargin {
				opp.Viable = true
				log.Printf("   🎯 VIABLE: %s → %s (%.2f%% net margin)",
					buySymbol, sellSymbol, opp.NetMarginPct)
			} else {
				log.Printf("   ❌ %s → %s: %.2f%% margin (below %.1f%% threshold)",
					buySymbol, sellSymbol, opp.NetMarginPct, config.MinNetMargin)
			}

			opportunities = append(opportunities, opp)
		}
	}

	return opportunities, nil
}

type PriceInfo struct {
	Pair         types.PairInfo
	BestBid      float64
	BestAsk      float64
	BidVolume    float64
	AskVolume    float64
	BestBidINR   float64
	BestAskINR   float64
	HasLiquidity bool
}

func getPriceInfo(pair types.PairInfo, fetcher *market.Fetcher, rateManager *exchange.RateManager) (PriceInfo, error) {
	orderBook, err := fetcher.GetOrderBook(pair.Pair)
	if err != nil {
		return PriceInfo{}, err
	}

	priceInfo := PriceInfo{Pair: pair}

	// Parse bids (buy orders)
	if bids, ok := orderBook["bids"].(map[string]interface{}); ok {
		for priceStr, volumeInterface := range bids {
			price, _ := strconv.ParseFloat(priceStr, 64)
			var volume float64
			switch v := volumeInterface.(type) {
			case string:
				volume, _ = strconv.ParseFloat(v, 64)
			case float64:
				volume = v
			}

			if price > priceInfo.BestBid {
				priceInfo.BestBid = price
				priceInfo.BidVolume = volume
			}
		}
	}

	// Parse asks (sell orders)
	priceInfo.BestAsk = 999999999.0
	if asks, ok := orderBook["asks"].(map[string]interface{}); ok {
		for priceStr, volumeInterface := range asks {
			price, _ := strconv.ParseFloat(priceStr, 64)
			var volume float64
			switch v := volumeInterface.(type) {
			case string:
				volume, _ = strconv.ParseFloat(v, 64)
			case float64:
				volume = v
			}

			if price < priceInfo.BestAsk {
				priceInfo.BestAsk = price
				priceInfo.AskVolume = volume
			}
		}
	}

	// Convert to INR
	if priceInfo.BestBid > 0 {
		priceInfo.BestBidINR, _ = rateManager.ConvertToINR(priceInfo.BestBid, pair.BaseCurrency)
	}
	if priceInfo.BestAsk < 999999999.0 {
		priceInfo.BestAskINR, _ = rateManager.ConvertToINR(priceInfo.BestAsk, pair.BaseCurrency)
	}

	return priceInfo, nil
}

func calculateArbitrage(currency string, buyPrice, sellPrice PriceInfo, config *types.Config) types.ArbitrageOpportunity {
	// Calculate margins in INR terms
	grossMargin := sellPrice.BestBidINR - buyPrice.BestAskINR
	grossMarginPct := (grossMargin / buyPrice.BestAskINR) * 100

	// Estimate fees
	estimatedFees := (buyPrice.BestAskINR + sellPrice.BestBidINR) * config.FeeRate

	// Calculate net margins
	netMargin := grossMargin - estimatedFees
	netMarginPct := (netMargin / buyPrice.BestAskINR) * 100

	return types.ArbitrageOpportunity{
		TargetCurrency: currency,
		BuyMarket: struct {
			Symbol       string `json:"symbol"`
			Pair         string `json:"pair"`
			BaseCurrency string `json:"base_currency"`
		}{
			Symbol:       buyPrice.Pair.Symbol,
			Pair:         buyPrice.Pair.Pair,
			BaseCurrency: buyPrice.Pair.BaseCurrency,
		},
		SellMarket: struct {
			Symbol       string `json:"symbol"`
			Pair         string `json:"pair"`
			BaseCurrency string `json:"base_currency"`
		}{
			Symbol:       sellPrice.Pair.Symbol,
			Pair:         sellPrice.Pair.Pair,
			BaseCurrency: sellPrice.Pair.BaseCurrency,
		},
		BuyPriceINR:    buyPrice.BestAskINR,
		SellPriceINR:   sellPrice.BestBidINR,
		GrossMargin:    grossMargin,
		GrossMarginPct: grossMarginPct,
		EstimatedFees:  estimatedFees,
		NetMargin:      netMargin,
		NetMarginPct:   netMarginPct,
		Viable:         false, // Set by caller
		Timestamp:      time.Now(),
	}
}

func executeOpportunity(engine *arbitrage.Engine, opp types.ArbitrageOpportunity, oppNumber int) {
	defer wg.Done()

	opportunityID := fmt.Sprintf("%s_%s_%s", opp.TargetCurrency,
		opp.BuyMarket.Symbol, opp.SellMarket.Symbol)

	log.Printf("⏳ [%d] %s: Waiting for execution lock...", oppNumber, opportunityID)

	// 🔒 ACQUIRE GLOBAL EXECUTION LOCK
	executionMutex.Lock()
	defer executionMutex.Unlock()

	log.Printf("🚀 [%d] %s: Execution lock acquired, starting execution...", oppNumber, opportunityID)

	// Execute with single opportunity
	singleOppSlice := []types.ArbitrageOpportunity{opp}
	result, err := engine.Execute(singleOppSlice)
	if err != nil {
		log.Printf("❌ [%d] %s: Execution failed: %v", oppNumber, opportunityID, err)
		return
	}

	// Log results
	if result.Successful && len(result.Orders) > 0 {
		order := result.Orders[0]
		log.Printf("💰 [%d] %s: SUCCESS - ₹%.2f profit (%.2f%%) in %dms",
			oppNumber, opportunityID, order.ActualProfit, order.ActualMarginPct, order.ExecutionTimeMs)
	} else {
		log.Printf("❌ [%d] %s: Execution completed but no profit", oppNumber, opportunityID)
	}

	// Save execution log
	filename := fmt.Sprintf("execution_log_%s_%d.json", opportunityID, result.Timestamp.Unix())
	err = engine.SaveExecutionLog(result, filename)
	if err != nil {
		log.Printf("⚠️ [%d] %s: Error saving execution log: %v", oppNumber, opportunityID, err)
	}

	log.Printf("✅ [%d] %s: Execution complete, lock released", oppNumber, opportunityID)
}

// Helper function to check if opportunity involves USDT
func hasUSDTPair(opp types.ArbitrageOpportunity) bool {
	return strings.Contains(opp.BuyMarket.Symbol, "USDT") ||
		strings.Contains(opp.SellMarket.Symbol, "USDT")
}

func parseFloat(s string) float64 {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return val
}
