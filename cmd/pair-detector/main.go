package main

import (
	"fmt"
	"log"
	"os"

	"github.com/b-thark/cdcx-api/pkg/pairs"
	"github.com/b-thark/cdcx-api/pkg/types"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🔍 CoinDCX Arbitrage Pair Detector")
	fmt.Println("==================================")
	fmt.Println("🎯 Finding currencies with multiple trading pairs for arbitrage")

	// Load configuration
	config := types.DefaultConfig()

	// Allow user to enable all pairs via environment variable
	if os.Getenv("ENABLE_ALL_PAIRS") == "true" {
		config.EnableAllPairs = true
		fmt.Println("🌐 ALL PAIRS MODE: Including all base currencies")
	} else {
		fmt.Printf("🔒 FILTERED MODE: Only including %v\n", config.ValidCurrencies)
		fmt.Println("💡 Set ENABLE_ALL_PAIRS=true to include all currencies")
	}

	// Create analyzer
	analyzer := pairs.NewAnalyzer(config)

	// Extract arbitrage pairs
	fmt.Println("\n📊 Extracting arbitrage pairs...")
	arbitragePairs, err := analyzer.ExtractArbitragePairs()
	if err != nil {
		log.Fatalf("❌ Error extracting pairs: %v", err)
	}

	// Display results
	analyzer.DisplaySummary(arbitragePairs)

	// Save pairs to file
	filename := "arbitrage_pairs.json"
	err = analyzer.SavePairs(arbitragePairs, filename)
	if err != nil {
		log.Fatalf("❌ Error saving pairs: %v", err)
	}

	fmt.Printf("\n💾 Saved arbitrage pairs to %s\n", filename)
	fmt.Printf("🚀 Ready for opportunity detection! Run: go run cmd/opportunity-detector/main.go\n")
}
