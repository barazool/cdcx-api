package main

import (
	"fmt"
	"os"
	"time"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/arbitrage"
	"github.com/b-thark/cdcx-api/pkg/coindcx"
)

func main() {
	fmt.Println("🚀 CoinDCX Simple Arbitrage Detector")
	fmt.Println("====================================")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("❌ Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create CoinDCX client
	client := coindcx.NewClient(cfg.APIKey, cfg.APISecret)

	// Create arbitrage detector
	detector := arbitrage.NewDetector(client)

	// Update trading context with current user data
	fmt.Println("\n📋 Updating trading context...")
	err = detector.UpdateContext()
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not update context: %v\n", err)
		fmt.Println("   Proceeding with default settings...")
	}

	// Analyze all markets to find potential opportunities
	fmt.Println("\n🔍 Analyzing markets for simple arbitrage opportunities...")
	fmt.Println("   Looking for: USDT → COIN → (INR/BTC/ETH)")
	startTime := time.Now()

	matrix, err := detector.AnalyzeMarkets()
	if err != nil {
		fmt.Printf("❌ Error analyzing markets: %v\n", err)
		os.Exit(1)
	}

	analysisTime := time.Since(startTime)
	fmt.Printf("\n📊 Market Analysis Complete (took %v)\n", analysisTime)
	fmt.Printf("   Total Markets: %d\n", matrix.TotalPairs)
	fmt.Printf("   USDT Pairs: %d\n", len(matrix.USDTPairs))
	for currency, pairs := range matrix.TargetPairs {
		fmt.Printf("   %s Pairs: %d\n", currency, len(pairs))
	}
	fmt.Printf("   Potential Opportunities: %d\n", matrix.TotalOpportunities)

	// If no opportunities found, exit
	if matrix.TotalOpportunities == 0 {
		fmt.Println("\n😔 No arbitrage opportunities found")
		fmt.Println("   This could be because:")
		fmt.Println("   • No coins have both USDT and INR/BTC/ETH pairs")
		fmt.Println("   • Markets are inactive or delisted")
		fmt.Println("   • API limitations or temporary issues")
		return
	}

	// Analyze prices for opportunities (limit to first 20 to avoid rate limits)
	fmt.Println("\n💹 Analyzing prices for opportunities...")
	fmt.Println("   ⚠️  Note: Limiting to first 20 opportunities to avoid API rate limits")

	opportunitiesToAnalyze := matrix.Opportunities
	if len(opportunitiesToAnalyze) > 20 {
		opportunitiesToAnalyze = opportunitiesToAnalyze[:20]
	}

	viableOpportunities, err := detector.AnalyzePrices(opportunitiesToAnalyze)
	if err != nil {
		fmt.Printf("❌ Error analyzing prices: %v\n", err)
		os.Exit(1)
	}

	// Display results
	if len(viableOpportunities) == 0 {
		fmt.Println("\n😔 No viable arbitrage opportunities found")
		fmt.Println("   This means:")
		fmt.Println("   • Price differences exist but are smaller than trading costs")
		fmt.Println("   • Insufficient liquidity at profitable price levels")
		fmt.Println("   • Markets are currently efficient")
		fmt.Println("   • 2% minimum ROI threshold not met")
		return
	}

	// Get top opportunities
	topOpportunities := detector.GetTopOpportunities(viableOpportunities, 5)

	fmt.Printf("\n🎯 TOP %d ARBITRAGE OPPORTUNITIES\n", len(topOpportunities))
	fmt.Println("==========================================")

	// Display summary table
	fmt.Printf("\n%-6s %-10s %-15s %-8s %-10s %-10s %-12s\n",
		"Rank", "Coin", "Path", "ROI %", "Profit ₹", "Volume", "Min Invest ₹")
	fmt.Println("------------------------------------------------------------------------")

	for i, opp := range topOpportunities {
		pathStr := fmt.Sprintf("USDT→%s", opp.TargetCurrency)
		fmt.Printf("%-6d %-10s %-15s %-8.2f %-10.2f %-10.2f %-12.2f\n",
			i+1, opp.Coin, pathStr, opp.ROI, opp.FinalProfit,
			opp.MaxTradeVolume, opp.MinInvestment)
	}

	// Display detailed analysis for top 3 opportunities
	fmt.Println("\n📋 DETAILED ANALYSIS")
	fmt.Println("====================")
	detailLimit := 3
	if len(topOpportunities) < detailLimit {
		detailLimit = len(topOpportunities)
	}

	for i := 0; i < detailLimit; i++ {
		detector.PrintOpportunityDetails(topOpportunities[i])
	}

	// Display trading strategy explanation
	fmt.Println("\n📚 SIMPLE ARBITRAGE EXPLAINED:")
	fmt.Println("   1. Buy cryptocurrency with USDT")
	fmt.Println("   2. Sell same cryptocurrency for INR/BTC/ETH")
	fmt.Println("   3. Profit = Price difference - Trading fees - Taxes")

	// Display important disclaimers
	fmt.Println("\n⚠️  IMPORTANT DISCLAIMERS:")
	fmt.Println("   • These are theoretical opportunities based on current orderbook data")
	fmt.Println("   • Prices change rapidly; opportunities may disappear quickly")
	fmt.Println("   • Market slippage not considered in calculations")
	fmt.Println("   • Tax calculations are estimates; consult a tax advisor")
	fmt.Println("   • 30% capital gains tax applies to all profits")
	fmt.Println("   • 1% TDS applies to INR conversions above threshold")
	fmt.Println("   • Always verify prices before executing trades")
	fmt.Println("   • Start with small amounts to test the strategy")

	fmt.Println("\n💡 EXECUTION TIPS:")
	fmt.Println("   • Use limit orders to avoid slippage")
	fmt.Println("   • Monitor orderbook depth before large trades")
	fmt.Println("   • Consider transaction fees and confirmation times")
	fmt.Println("   • Keep some buffer for price movements")

	fmt.Println("\n✅ Simple Arbitrage Analysis Complete!")
	fmt.Printf("   Found %d viable opportunities out of %d analyzed\n",
		len(viableOpportunities), len(opportunitiesToAnalyze))
	fmt.Printf("   Total analysis time: %v\n", time.Since(startTime))
	fmt.Printf("   Analysis timestamp: %v\n", time.Unix(matrix.AnalysisTimestamp, 0).Format("2006-01-02 15:04:05"))
}
