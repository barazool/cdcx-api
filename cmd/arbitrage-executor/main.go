package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/executor"
	"github.com/b-thark/cdcx-api/pkg/types"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🚀 CoinDCX Arbitrage Executor")
	fmt.Println("=============================")
	fmt.Println("⚠️  LIVE TRADING MODE - REAL EXECUTION")
	fmt.Println("💰 Executing profitable arbitrage opportunities...")

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

	// Create executor
	arbitrageExecutor := executor.NewArbitrageExecutor(cfg, execConfig)

	// Load depth analysis results
	fmt.Println("\n📂 Loading depth analysis results...")
	analyses, err := arbitrageExecutor.LoadAnalyses("depth_analysis.json")
	if err != nil {
		log.Fatalf("❌ Error loading analyses: %v\n💡 Run depth analyzer first: go run cmd/depth-analyzer/main.go", err)
	}

	if len(analyses) == 0 {
		fmt.Println("❌ No profitable opportunities found in analysis")
		return
	}

	fmt.Printf("✅ Loaded %d profitable opportunities\n", len(analyses))

	// Check account readiness
	fmt.Println("\n🔍 Checking account status...")
	ready, err := arbitrageExecutor.CheckAccountReadiness()
	if err != nil {
		log.Fatalf("❌ Account check failed: %v", err)
	}

	if !ready {
		fmt.Println("❌ Account not ready for execution")
		return
	}

	fmt.Println("✅ Account ready for execution")

	// Display execution plan
	fmt.Println("\n📋 EXECUTION PLAN:")
	fmt.Println("==================")
	arbitrageExecutor.DisplayExecutionPlan(analyses)

	// Execute arbitrage
	fmt.Println("\n🚀 Starting arbitrage execution...")
	results, err := arbitrageExecutor.ExecuteArbitrage(analyses)
	if err != nil {
		log.Fatalf("❌ Execution failed: %v", err)
	}

	// Display results
	fmt.Println("\n📊 EXECUTION RESULTS:")
	fmt.Println("====================")
	arbitrageExecutor.DisplayResults(results)

	// Save execution log
	filename := fmt.Sprintf("execution_log_%d.json", results.Timestamp.Unix())
	err = arbitrageExecutor.SaveExecutionLog(results, filename)
	if err != nil {
		log.Printf("⚠️ Error saving execution log: %v", err)
	} else {
		fmt.Printf("\n💾 Execution log saved to %s\n", filename)
	}

	fmt.Println("\n🎯 Execution complete!")
}

func parseFloat(s string) float64 {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("⚠️ Error parsing float '%s': %v", s, err)
		return 0.0
	}
	return val
}
