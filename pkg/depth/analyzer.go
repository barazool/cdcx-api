package depth

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/b-thark/cdcx-api/pkg/exchange"
	"github.com/b-thark/cdcx-api/pkg/market"
	"github.com/b-thark/cdcx-api/pkg/types"
	"github.com/b-thark/cdcx-api/pkg/utils"
)

type Analyzer struct {
	fetcher     *market.Fetcher
	rateManager *exchange.RateManager
	config      *types.Config
}

func NewAnalyzer(config *types.Config) *Analyzer {
	return &Analyzer{
		fetcher:     market.NewFetcher(),
		rateManager: exchange.NewRateManager(config),
		config:      config,
	}
}

func (a *Analyzer) AnalyzeDepth(opportunities []types.ArbitrageOpportunity) ([]types.ArbitrageDepthAnalysis, error) {
	log.Println("🔬 Starting order book depth analysis...")

	// Filter only viable opportunities
	viableOpps := []types.ArbitrageOpportunity{}
	for _, opp := range opportunities {
		if opp.Viable {
			viableOpps = append(viableOpps, opp)
		}
	}

	if len(viableOpps) == 0 {
		return nil, fmt.Errorf("no viable opportunities to analyze")
	}

	log.Printf("📊 Analyzing depth for %d viable opportunities...", len(viableOpps))

	analyses := []types.ArbitrageDepthAnalysis{}

	for _, opp := range viableOpps {
		log.Printf("🔍 Analyzing %s: %s → %s",
			opp.TargetCurrency, opp.BuyMarket.Symbol, opp.SellMarket.Symbol)

		analysis, err := a.analyzeOpportunityDepth(opp)
		if err != nil {
			log.Printf("❌ %s: %v", opp.TargetCurrency, err)
			continue
		}

		if analysis.MaxProfitableOrders > 0 {
			analyses = append(analyses, analysis)
			log.Printf("✅ %s: %d profitable orders, ₹%.2f total profit",
				opp.TargetCurrency, analysis.MaxProfitableOrders, analysis.TotalEstimatedProfit)
		} else {
			log.Printf("⚠️ %s: No profitable depth found", opp.TargetCurrency)
		}
	}

	// Save rate cache
	a.rateManager.SaveCache()

	return analyses, nil
}

func (a *Analyzer) analyzeOpportunityDepth(opp types.ArbitrageOpportunity) (types.ArbitrageDepthAnalysis, error) {
	// Create PairInfo from opportunity data with base currencies
	buyPair := types.PairInfo{
		Symbol:         opp.BuyMarket.Symbol,
		Pair:           opp.BuyMarket.Pair,
		BaseCurrency:   opp.BuyMarket.BaseCurrency,
		TargetCurrency: opp.TargetCurrency,
	}
	sellPair := types.PairInfo{
		Symbol:         opp.SellMarket.Symbol,
		Pair:           opp.SellMarket.Pair,
		BaseCurrency:   opp.SellMarket.BaseCurrency,
		TargetCurrency: opp.TargetCurrency,
	}

	// Get detailed order books
	buyOrderBook, err := a.getEnhancedOrderBook(buyPair)
	if err != nil {
		return types.ArbitrageDepthAnalysis{}, fmt.Errorf("buy order book error: %v", err)
	}

	sellOrderBook, err := a.getEnhancedOrderBook(sellPair)
	if err != nil {
		return types.ArbitrageDepthAnalysis{}, fmt.Errorf("sell order book error: %v", err)
	}

	// Simulate step-by-step execution
	return a.simulateArbitrageDepth(opp.TargetCurrency, buyOrderBook, sellOrderBook), nil
}

func (a *Analyzer) getEnhancedOrderBook(pair types.PairInfo) (types.EnhancedOrderBook, error) {
	rawOrderBook, err := a.fetcher.GetOrderBook(pair.Pair)
	if err != nil {
		return types.EnhancedOrderBook{}, err
	}

	orderBook := types.EnhancedOrderBook{
		Symbol:       pair.Symbol,
		Pair:         pair.Pair,
		BaseCurrency: pair.BaseCurrency,
		Timestamp:    time.Now(),
	}

	// Process bids
	if bids, ok := rawOrderBook["bids"].(map[string]interface{}); ok {
		orderBook.BidLevels = a.processOrderBookSide(bids, pair.BaseCurrency, "bid")
		if len(orderBook.BidLevels) > 0 {
			orderBook.BestBid = orderBook.BidLevels[0].Price
			orderBook.BestBidINR = orderBook.BidLevels[0].PriceINR
		}
	}

	// Process asks
	if asks, ok := rawOrderBook["asks"].(map[string]interface{}); ok {
		orderBook.AskLevels = a.processOrderBookSide(asks, pair.BaseCurrency, "ask")
		if len(orderBook.AskLevels) > 0 {
			orderBook.BestAsk = orderBook.AskLevels[0].Price
			orderBook.BestAskINR = orderBook.AskLevels[0].PriceINR
		}
	}

	// Calculate spread and totals
	if orderBook.BestBid > 0 && orderBook.BestAsk > 0 {
		orderBook.Spread = orderBook.BestAsk - orderBook.BestBid
		orderBook.SpreadPct = (orderBook.Spread / orderBook.BestAsk) * 100
	}

	for _, level := range orderBook.BidLevels {
		orderBook.TotalBidVolume += level.Volume
	}
	for _, level := range orderBook.AskLevels {
		orderBook.TotalAskVolume += level.Volume
	}

	return orderBook, nil
}

func (a *Analyzer) processOrderBookSide(orders map[string]interface{}, baseCurrency, side string) []types.OrderBookLevel {
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

	// Sort levels
	if side == "bid" {
		sort.Slice(levels, func(i, j int) bool {
			return levels[i].price > levels[j].price
		})
	} else {
		sort.Slice(levels, func(i, j int) bool {
			return levels[i].price < levels[j].price
		})
	}

	// Convert to enhanced levels
	enhanced := []types.OrderBookLevel{}
	cumulative := 0.0

	maxLevels := a.config.MaxOrderLevels
	if len(levels) < maxLevels {
		maxLevels = len(levels)
	}

	for i := 0; i < maxLevels; i++ {
		level := levels[i]

		priceINR, err := a.rateManager.ConvertToINR(level.price, baseCurrency)
		if err != nil {
			log.Printf("      ⚠️ Price conversion failed for %f %s: %v", level.price, baseCurrency, err)
			continue
		}

		cumulative += level.volume

		enhanced = append(enhanced, types.OrderBookLevel{
			Price:      level.price,
			Volume:     level.volume,
			PriceINR:   priceINR,
			Cumulative: cumulative,
			VolumeINR:  level.volume * priceINR,
		})
	}

	return enhanced
}

func (a *Analyzer) simulateArbitrageDepth(currency string, buyMarket, sellMarket types.EnhancedOrderBook) types.ArbitrageDepthAnalysis {
	log.Printf("   🧮 SIMULATING: %s", currency)
	log.Printf("      🟢 BUY from %s (best: ₹%.4f)", buyMarket.Symbol, buyMarket.BestAskINR)
	log.Printf("      🔴 SELL to %s (best: ₹%.4f)", sellMarket.Symbol, sellMarket.BestBidINR)

	analysis := types.ArbitrageDepthAnalysis{
		Currency:   currency,
		BuyMarket:  buyMarket,
		SellMarket: sellMarket,
		Timestamp:  time.Now(),
	}

	if buyMarket.BestAskINR >= sellMarket.BestBidINR {
		log.Printf("      ❌ No arbitrage: buy ₹%.4f >= sell ₹%.4f",
			buyMarket.BestAskINR, sellMarket.BestBidINR)
		return analysis
	}

	log.Printf("      ✅ Initial margin: ₹%.4f (%.2f%%)",
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
		estimatedFees := tradeValueINR * a.config.FeeRate
		netMargin := (grossMargin * tradeableVolume) - estimatedFees
		netMarginPct := (netMargin / tradeValueINR) * 100

		log.Printf("      📋 Order %d: Vol %.4f, Buy ₹%.4f, Sell ₹%.4f, Net %.2f%%",
			orderNumber, tradeableVolume, buyPriceINR, sellPriceINR, netMarginPct)

		// Check if still profitable
		profitable := netMarginPct >= a.config.MinNetMargin

		if profitable {
			cumulativeVolume += tradeableVolume
			cumulativeVolumeINR += tradeValueINR
			cumulativeNetProfit += netMargin

			simulation := types.OrderSimulation{
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

			log.Printf("         ✅ Profitable! Net: ₹%.2f, Cumulative: ₹%.2f", netMargin, cumulativeNetProfit)
		} else {
			log.Printf("         ❌ No longer profitable (%.2f%% < %.1f%%)", netMarginPct, a.config.MinNetMargin)
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

	log.Printf("      🎯 RESULT: %d profitable orders, ₹%.2f total profit, %s rating",
		analysis.MaxProfitableOrders, analysis.TotalEstimatedProfit, analysis.OpportunityRating)

	return analysis
}

func (a *Analyzer) SaveAnalyses(analyses []types.ArbitrageDepthAnalysis, filename string) error {
	return utils.SaveJSON(analyses, filename)
}

func (a *Analyzer) LoadAnalyses(filename string) ([]types.ArbitrageDepthAnalysis, error) {
	var analyses []types.ArbitrageDepthAnalysis
	err := utils.LoadJSON(filename, &analyses)
	return analyses, err
}

func (a *Analyzer) DisplayResults(analyses []types.ArbitrageDepthAnalysis) {
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
		fmt.Printf("   📊 Max Orders: %d | Total Volume: %.4f tokens\n",
			analysis.MaxProfitableOrders, analysis.TotalProfitableVolume)

		if len(analysis.OrderSimulations) > 0 {
			lastSim := analysis.OrderSimulations[len(analysis.OrderSimulations)-1]
			fmt.Printf("   💰 Total Value: ₹%.2f | Total Profit: ₹%.2f\n",
				lastSim.Cumulative.VolumeINR, analysis.TotalEstimatedProfit)
		}

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

	// Display summary statistics
	fmt.Printf("\n📈 DEPTH ANALYSIS SUMMARY:\n")
	fmt.Printf("=========================\n")

	totalProfit := 0.0
	totalVolume := 0.0
	avgOrders := 0.0

	ratingCount := make(map[string]int)

	for _, analysis := range analyses {
		totalProfit += analysis.TotalEstimatedProfit
		totalVolume += analysis.TotalProfitableVolume
		avgOrders += float64(analysis.MaxProfitableOrders)
		ratingCount[analysis.OpportunityRating]++
	}

	if len(analyses) > 0 {
		avgOrders /= float64(len(analyses))
	}

	fmt.Printf("📊 Total Estimated Profit: ₹%.2f\n", totalProfit)
	fmt.Printf("📊 Total Volume: %.4f tokens\n", totalVolume)
	fmt.Printf("📊 Average Orders per Opportunity: %.1f\n", avgOrders)
	fmt.Printf("📊 Rating Distribution:\n")
	for rating, count := range ratingCount {
		fmt.Printf("   %s: %d opportunities\n", rating, count)
	}
}
