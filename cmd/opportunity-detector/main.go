package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/b-thark/cdcx-api/pkg/opportunity"
	"github.com/b-thark/cdcx-api/pkg/pairs"
	"github.com/b-thark/cdcx-api/pkg/types"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🚀 CoinDCX Arbitrage Opportunity Detector")
	fmt.Println("=========================================")
	fmt.Println("💡 Analyzing real-time prices for arbitrage opportunities")

	// Load configuration
	config := types.DefaultConfig()

	// Allow configuration override via environment variables
	if minMargin := os.Getenv("MIN_NET_MARGIN"); minMargin != "" {
		if margin := parseFloat(minMargin); margin > 0 {
			config.MinNetMargin = margin
			fmt.Printf("🎯 Custom minimum net margin: %.1f%%\n", margin)
		}
	}

	if minLiquidity := os.Getenv("MIN_LIQUIDITY"); minLiquidity != "" {
		if liquidity := parseFloat(minLiquidity); liquidity > 0 {
			config.MinLiquidity = liquidity
			fmt.Printf("💧 Custom minimum liquidity: ₹%.2f\n", liquidity)
		}
	}

	// Load arbitrage pairs
	fmt.Println("\n📂 Loading arbitrage pairs...")
	pairAnalyzer := pairs.NewAnalyzer(config)
	arbitragePairs, err := pairAnalyzer.LoadPairs("arbitrage_pairs.json")
	if err != nil {
		log.Fatalf("❌ Error loading pairs: %v\n💡 Run pair detector first: go run cmd/pair-detector/main.go", err)
	}

	fmt.Printf("✅ Loaded %d currencies with arbitrage potential\n", len(arbitragePairs))

	// Create opportunity detector
	detector := opportunity.NewDetector(config)

	// Find opportunities
	fmt.Println("\n🔍 Analyzing arbitrage opportunities...")
	opportunities, err := detector.FindOpportunities(arbitragePairs)
	if err != nil {
		log.Fatalf("❌ Error finding opportunities: %v", err)
	}

	// Display results
	detector.DisplayResults(opportunities)

	// Save opportunities to file
	filename := "arbitrage_opportunities.json"
	err = detector.SaveOpportunities(opportunities, filename)
	if err != nil {
		log.Fatalf("❌ Error saving opportunities: %v", err)
	}

	fmt.Printf("\n💾 Saved opportunities to %s\n", filename)
	fmt.Printf("🔬 Ready for depth analysis! Run: go run cmd/depth-analyzer/main.go\n")
}

func parseFloat(s string) float64 {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return val
}
