package main

import (
	"fmt"
	"log"

	"github.com/b-thark/cdcx-api/pkg/depth"
	"github.com/b-thark/cdcx-api/pkg/opportunity"
	"github.com/b-thark/cdcx-api/pkg/types"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🔬 CoinDCX Order Book Depth Analyzer")
	fmt.Println("====================================")
	fmt.Println("⚠️  ANALYSIS MODE - NO EXECUTION")
	fmt.Println("🔍 Deep diving into profitable arbitrage opportunities...")

	// Load configuration
	config := types.DefaultConfig()

	// Load opportunities from previous analysis
	fmt.Println("\n📂 Loading arbitrage opportunities...")
	oppDetector := opportunity.NewDetector(config)
	opportunities, err := oppDetector.LoadOpportunities("arbitrage_opportunities.json")
	if err != nil {
		log.Fatalf("❌ Error loading opportunities: %v\n💡 Run opportunity detector first: go run cmd/opportunity-detector/main.go", err)
	}

	// Count viable opportunities
	viableCount := 0
	for _, opp := range opportunities {
		if opp.Viable {
			viableCount++
		}
	}

	fmt.Printf("✅ Loaded %d total opportunities (%d viable)\n", len(opportunities), viableCount)

	if viableCount == 0 {
		fmt.Println("❌ No viable opportunities found for depth analysis")
		fmt.Println("💡 Try lowering the minimum net margin or running opportunity detector again")
		return
	}

	// Create depth analyzer
	analyzer := depth.NewAnalyzer(config)

	// Analyze depth
	fmt.Println("\n🔍 Analyzing order book depth...")
	analyses, err := analyzer.AnalyzeDepth(opportunities)
	if err != nil {
		log.Fatalf("❌ Error analyzing depth: %v", err)
	}

	// Display results
	analyzer.DisplayResults(analyses)

	// Save detailed analysis
	filename := "depth_analysis.json"
	err = analyzer.SaveAnalyses(analyses, filename)
	if err != nil {
		log.Fatalf("❌ Error saving analysis: %v", err)
	}

	fmt.Printf("\n💾 Saved detailed depth analysis to %s\n", filename)

	if len(analyses) > 0 {
		fmt.Println("🎯 Analysis complete! Review the results above for execution strategy.")
		fmt.Println("⚠️  Remember: This is analysis only - no actual trades were executed.")
	} else {
		fmt.Println("📉 No opportunities with sufficient order book depth found.")
		fmt.Println("💡 Consider adjusting minimum margin or liquidity thresholds.")
	}
}
