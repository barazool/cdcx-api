package opportunity

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/arbitrage"
	"github.com/b-thark/cdcx-api/pkg/types"
)

type LiveDetector struct {
	*Detector
	engine       *arbitrage.Engine
	execConfig   *types.ExecutionConfig
	executionMux sync.Mutex // Single execution lock
	activeJobs   sync.Map   // Track active detection jobs
}

func NewLiveDetector(tradingConfig *types.Config, apiConfig *config.Config, execConfig *types.ExecutionConfig) *LiveDetector {
	return &LiveDetector{
		Detector:   NewDetector(tradingConfig),
		engine:     arbitrage.NewEngine(apiConfig, execConfig),
		execConfig: execConfig,
	}
}

func (ld *LiveDetector) FindAndExecuteOpportunities(pairs map[string]types.ArbitragePairs) error {
	log.Println("🔍 Starting live arbitrage detection with sequential execution...")

	// Check account readiness once
	ready, err := ld.engine.CheckAccountReadiness()
	if err != nil {
		return fmt.Errorf("account check failed: %v", err)
	}
	if !ready {
		return fmt.Errorf("account not ready for execution")
	}

	var wg sync.WaitGroup

	for currency, pairGroup := range pairs {
		if len(pairGroup.Pairs) < 2 {
			continue
		}

		// Launch detection goroutine for each currency
		wg.Add(1)
		go func(curr string, pairs []types.PairInfo) {
			defer wg.Done()
			ld.detectAndExecute(curr, pairs)
		}(currency, pairGroup.Pairs)
	}

	// Wait for all detection goroutines to complete
	wg.Wait()
	log.Println("🎯 All detection and execution completed")
	return nil
}

func (ld *LiveDetector) detectAndExecute(currency string, pairs []types.PairInfo) {
	// Check if already processing this currency
	if _, exists := ld.activeJobs.LoadOrStore(currency, true); exists {
		log.Printf("⚠️ %s already being processed, skipping", currency)
		return
	}
	defer ld.activeJobs.Delete(currency)

	log.Printf("🔍 [%s] Analyzing opportunities...", currency)

	// Analyze currency for opportunities
	opportunities, err := ld.analyzeCurrency(currency, pairs)
	if err != nil {
		log.Printf("❌ [%s] Analysis failed: %v", currency, err)
		return
	}

	// Find viable opportunities
	viableOpps := []types.ArbitrageOpportunity{}
	for _, opp := range opportunities {
		if opp.Viable {
			viableOpps = append(viableOpps, opp)
		}
	}

	if len(viableOpps) == 0 {
		log.Printf("📉 [%s] No viable opportunities found", currency)
		return
	}

	log.Printf("✅ [%s] Found %d viable opportunities, attempting execution...",
		currency, len(viableOpps))

	// 🔒 ACQUIRE EXECUTION LOCK - Only one execution at a time
	log.Printf("⏳ [%s] Waiting for execution lock...", currency)
	ld.executionMux.Lock()
	defer ld.executionMux.Unlock()

	log.Printf("🚀 [%s] Execution lock acquired, starting execution...", currency)

	// Execute using the same logic as arbitrage engine
	result := ld.executeArbitrageSequentially(viableOpps)

	// Save execution log
	if result != nil {
		filename := fmt.Sprintf("execution_log_%s_%d.json", currency, time.Now().Unix())
		err := ld.engine.SaveExecutionLog(result, filename)
		if err != nil {
			log.Printf("⚠️ [%s] Error saving execution log: %v", currency, err)
		}

		// Log final results
		if result.Successful && len(result.Orders) > 0 {
			log.Printf("💰 [%s] EXECUTION COMPLETE: ₹%.2f total profit from %d orders",
				currency, result.TotalProfit, len(result.Orders))
		} else {
			log.Printf("❌ [%s] Execution failed or no profit", currency)
		}
	}
}

func (ld *LiveDetector) executeArbitrageSequentially(opportunities []types.ArbitrageOpportunity) *types.ExecutionResult {
	// This is exactly the same as arbitrage.Engine.Execute()
	result := &types.ExecutionResult{
		StartTime:  time.Now(),
		Timestamp:  time.Now(),
		Successful: false,
		Orders:     []types.ExecutedOrder{},
		Config:     *ld.execConfig,
	}

	totalProfit := 0.0
	totalInvestment := 0.0
	processedCount := 0

	// Filter USDT pairs and sort by margin
	viableOpps := []types.ArbitrageOpportunity{}
	for _, opp := range opportunities {
		if opp.Viable && (strings.Contains(opp.BuyMarket.Symbol, "USDT") ||
			strings.Contains(opp.SellMarket.Symbol, "USDT")) {
			viableOpps = append(viableOpps, opp)
		}
	}

	// Sort by expected margin
	sort.Slice(viableOpps, func(i, j int) bool {
		return viableOpps[i].NetMarginPct > viableOpps[j].NetMarginPct
	})

	log.Printf("🔄 Processing %d USDT-paired opportunities...", len(viableOpps))

	for _, opp := range viableOpps {
		processedCount++
		log.Printf("📊 [%d/%d] Processing %s (%s → %s)",
			processedCount, len(viableOpps), opp.TargetCurrency,
			opp.BuyMarket.Symbol, opp.SellMarket.Symbol)

		// Real-time validation and execution (same as engine)
		liveOpp := ld.engine.AnalyzeAndValidateRealTime(opp)

		if !liveOpp.Viable {
			log.Printf("❌ %s: %s", opp.TargetCurrency, liveOpp.Reason)
			continue
		}

		log.Printf("✅ %s: %.2f%% margin - EXECUTING",
			opp.TargetCurrency, liveOpp.MarginPct)

		// Execute immediately
		executedOrder := ld.engine.ExecuteRealTimeOrder(liveOpp)
		result.Orders = append(result.Orders, executedOrder)

		if executedOrder.Success {
			totalProfit += executedOrder.ActualProfit
			totalInvestment += (executedOrder.VolumeExecuted * executedOrder.BuyPrice) / 83.0
			log.Printf("💰 %s SUCCESS: ₹%.2f profit", opp.TargetCurrency, executedOrder.ActualProfit)
		}

		// Check limits
		if totalInvestment >= ld.execConfig.MaxPositionUSDT {
			log.Printf("💰 Position limit reached: $%.2f", ld.execConfig.MaxPositionUSDT)
			break
		}

		// Delay between orders
		time.Sleep(time.Duration(ld.execConfig.DelayBetweenOrders) * time.Millisecond)
	}

	result.EndTime = time.Now()
	result.TotalProfit = totalProfit
	result.TotalInvestment = totalInvestment
	result.Successful = totalProfit > 0

	return result
}
