package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/arbitrage"
	"github.com/b-thark/cdcx-api/pkg/types"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🚀 CoinDCX Live Arbitrage Engine")
	fmt.Println("================================")
	fmt.Println("⚠️  LIVE TRADING MODE - REAL EXECUTION")
	fmt.Println("🔍 Real-time depth analysis + immediate execution")

	// Load API configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Error loading config: %v", err)
	}

	// Load execution configuration
	execConfig := types.DefaultExecutionConfig()

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

	// Create arbitrage engine
	engine := arbitrage.NewEngine(cfg, execConfig)

	// Load opportunities from previous analysis
	fmt.Println("\n📂 Loading arbitrage opportunities...")
	opportunities, err := engine.LoadOpportunities("arbitrage_opportunities.json")
	if err != nil {
		log.Fatalf("❌ Error loading opportunities: %v\n💡 Run opportunity detector first: go run cmd/opportunity-detector/main.go", err)
	}

	// Filter viable opportunities
	viableCount := 0
	for _, opp := range opportunities {
		if opp.Viable {
			viableCount++
		}
	}

	if viableCount == 0 {
		fmt.Println("❌ No viable opportunities found for execution")
		return
	}

	fmt.Printf("✅ Loaded %d viable opportunities\n", viableCount)

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

	// Display execution plan
	fmt.Println("\n📋 EXECUTION PLAN:")
	fmt.Println("==================")
	engine.DisplayExecutionPlan(opportunities)

	// Execute live arbitrage with real-time depth analysis
	fmt.Println("\n🚀 Starting live arbitrage execution...")
	results, err := engine.Execute(opportunities)
	if err != nil {
		log.Fatalf("❌ Execution failed: %v", err)
	}

	// Display results
	fmt.Println("\n📊 EXECUTION RESULTS:")
	fmt.Println("====================")
	engine.DisplayResults(results)

	// Save execution log
	filename := fmt.Sprintf("execution_log_%d.json", results.Timestamp.Unix())
	err = engine.SaveExecutionLog(results, filename)
	if err != nil {
		log.Printf("⚠️ Error saving execution log: %v", err)
	} else {
		fmt.Printf("\n💾 Execution log saved to %s\n", filename)
	}

	fmt.Println("\n🎯 Live arbitrage execution complete!")
}

func parseFloat(s string) float64 {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("⚠️ Error parsing float '%s': %v", s, err)
		return 0.0
	}
	return val
}
