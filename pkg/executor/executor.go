package executor

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/coindcx"
	"github.com/b-thark/cdcx-api/pkg/market"
	"github.com/b-thark/cdcx-api/pkg/types"
	"github.com/b-thark/cdcx-api/pkg/utils"
)

type ArbitrageExecutor struct {
	client    *coindcx.Client
	config    *types.ExecutionConfig
	apiConfig *config.Config
	fetcher   *market.Fetcher
	startTime time.Time
}

func NewArbitrageExecutor(apiConfig *config.Config, execConfig *types.ExecutionConfig) *ArbitrageExecutor {
	return &ArbitrageExecutor{
		client:    coindcx.NewClient(apiConfig.APIKey, apiConfig.APISecret),
		config:    execConfig,
		apiConfig: apiConfig,
		fetcher:   market.NewFetcher(),
		startTime: time.Now(),
	}
}

func (e *ArbitrageExecutor) LoadAnalyses(filename string) ([]types.ArbitrageDepthAnalysis, error) {
	var analyses []types.ArbitrageDepthAnalysis
	err := utils.LoadJSON(filename, &analyses)
	return analyses, err
}

func (e *ArbitrageExecutor) CheckAccountReadiness() (bool, error) {
	log.Println("🔍 Checking account balances...")

	balances, err := e.client.GetBalances()
	if err != nil {
		return false, fmt.Errorf("failed to get balances: %v", err)
	}

	usdtBalance := 0.0
	for _, balance := range balances {
		if balance.Currency == "USDT" {
			usdtBalance = balance.Balance
			break
		}
	}

	fmt.Printf("💰 Available USDT: %.6f\n", usdtBalance)

	if usdtBalance < e.config.MinRequiredUSDT {
		return false, fmt.Errorf("insufficient USDT balance: %.6f < %.6f required",
			usdtBalance, e.config.MinRequiredUSDT)
	}

	// Check if max position is within available balance
	if e.config.MaxPositionUSDT > usdtBalance*0.9 { // 90% of balance max
		e.config.MaxPositionUSDT = usdtBalance * 0.8 // Use 80% of balance
		fmt.Printf("⚠️ Adjusted max position to $%.2f (80%% of balance)\n", e.config.MaxPositionUSDT)
	}

	return true, nil
}

func (e *ArbitrageExecutor) DisplayExecutionPlan(analyses []types.ArbitrageDepthAnalysis) {
	fmt.Printf("🎯 Found %d opportunities to validate in real-time\n", len(analyses))
	fmt.Printf("   💰 Max Position: $%.2f USDT\n", e.config.MaxPositionUSDT)
	fmt.Printf("   🛑 Stop Loss: %.1f%%\n", e.config.StopLossPct)
}

type RealTimeOpportunity struct {
	Currency       string
	BuyMarket      string
	SellMarket     string
	BuyPrice       float64
	SellPrice      float64
	Volume         float64
	ExpectedMargin float64
	MarginPct      float64
	Viable         bool
	Reason         string
}

func (e *ArbitrageExecutor) ExecuteArbitrage(analyses []types.ArbitrageDepthAnalysis) (*types.ExecutionResult, error) {
	result := &types.ExecutionResult{
		StartTime:  time.Now(),
		Timestamp:  time.Now(),
		Successful: false,
		Orders:     []types.ExecutedOrder{},
		Config:     *e.config,
	}

	totalProfit := 0.0
	totalInvestment := 0.0

	// Real-time validation of opportunities
	fmt.Println("\n🔄 REAL-TIME MARKET VALIDATION:")
	fmt.Println("===============================")

	for _, analysis := range analyses {
		if !strings.Contains(analysis.BuyMarket.Symbol, "USDT") {
			continue
		}

		log.Printf("\n📊 Validating %s (%s → %s)",
			analysis.Currency, analysis.BuyMarket.Symbol, analysis.SellMarket.Symbol)

		// Get current real-time prices
		opportunity := e.validateOpportunityRealTime(analysis)

		if !opportunity.Viable {
			log.Printf("❌ %s: %s", analysis.Currency, opportunity.Reason)
			continue
		}

		log.Printf("✅ %s: %.2f%% margin VIABLE - EXECUTING NOW",
			analysis.Currency, opportunity.MarginPct)

		// Execute immediately while prices are good
		executedOrder := e.executeRealTimeOrder(opportunity)
		result.Orders = append(result.Orders, executedOrder)

		if executedOrder.Success {
			totalProfit += executedOrder.ActualProfit
			totalInvestment += (executedOrder.VolumeExecuted * executedOrder.BuyPrice) / 83.0
			log.Printf("💰 %s SUCCESS: ₹%.2f profit", analysis.Currency, executedOrder.ActualProfit)
		}

		// Check limits
		if totalInvestment >= e.config.MaxPositionUSDT {
			log.Printf("💰 Position limit reached: $%.2f", e.config.MaxPositionUSDT)
			break
		}

		// Small delay between executions
		time.Sleep(1 * time.Second)
	}

	result.EndTime = time.Now()
	result.TotalProfit = totalProfit
	result.TotalInvestment = totalInvestment
	result.Successful = totalProfit > 0

	return result, nil
}

func (e *ArbitrageExecutor) validateOpportunityRealTime(analysis types.ArbitrageDepthAnalysis) RealTimeOpportunity {
	opp := RealTimeOpportunity{
		Currency:   analysis.Currency,
		BuyMarket:  analysis.BuyMarket.Symbol,
		SellMarket: analysis.SellMarket.Symbol,
		Viable:     false,
	}

	// Get real-time prices for buy market
	buyOrderBook, err := e.fetcher.GetOrderBook(analysis.BuyMarket.Pair)
	if err != nil {
		opp.Reason = fmt.Sprintf("buy market data error: %v", err)
		return opp
	}

	// Get real-time prices for sell market
	sellOrderBook, err := e.fetcher.GetOrderBook(analysis.SellMarket.Pair)
	if err != nil {
		opp.Reason = fmt.Sprintf("sell market data error: %v", err)
		return opp
	}

	// Parse current buy price (we need to buy at ask price)
	buyPrice, buyVolume := e.getBestAsk(buyOrderBook)
	if buyPrice == 0 {
		opp.Reason = "no buy price available"
		return opp
	}

	// Parse current sell price (we need to sell at bid price)
	sellPrice, sellVolume := e.getBestBid(sellOrderBook)
	if sellPrice == 0 {
		opp.Reason = "no sell price available"
		return opp
	}

	// Convert to comparable currency (assume both in same base for now)
	// For proper comparison, would need exchange rate conversion
	opp.BuyPrice = buyPrice
	opp.SellPrice = sellPrice

	// Check if arbitrage is possible
	if sellPrice <= buyPrice {
		opp.Reason = fmt.Sprintf("no arbitrage: sell ₹%.6f <= buy ₹%.6f", sellPrice, buyPrice)
		return opp
	}

	// Calculate margin
	grossMargin := sellPrice - buyPrice
	grossMarginPct := (grossMargin / buyPrice) * 100

	// Estimate fees (2% total)
	estimatedFees := (buyPrice + sellPrice) * 0.01 // 1% each side roughly
	netMargin := grossMargin - estimatedFees
	netMarginPct := (netMargin / buyPrice) * 100

	opp.ExpectedMargin = netMargin
	opp.MarginPct = netMarginPct

	// Check minimum volume availability
	minVolume := 1000.0 // Minimum viable volume
	maxVolume := min(buyVolume, sellVolume)

	if maxVolume < minVolume {
		opp.Reason = fmt.Sprintf("insufficient volume: %.0f < %.0f required", maxVolume, minVolume)
		return opp
	}

	// Check if margin meets our threshold
	if netMarginPct < e.config.StopLossPct {
		opp.Reason = fmt.Sprintf("margin too low: %.2f%% < %.1f%% required", netMarginPct, e.config.StopLossPct)
		return opp
	}

	// Opportunity is viable
	opp.Volume = min(maxVolume, 5000.0) // Cap at reasonable volume
	opp.Viable = true
	opp.Reason = "profitable arbitrage detected"

	log.Printf("   💡 Current prices: Buy ₹%.6f, Sell ₹%.6f", buyPrice, sellPrice)
	log.Printf("   📊 Gross margin: ₹%.6f (%.2f%%)", grossMargin, grossMarginPct)
	log.Printf("   💸 Est. fees: ₹%.6f", estimatedFees)
	log.Printf("   💰 Net margin: ₹%.6f (%.2f%%)", netMargin, netMarginPct)
	log.Printf("   📈 Volume: %.0f tokens", opp.Volume)

	return opp
}

func (e *ArbitrageExecutor) getBestAsk(orderBook map[string]interface{}) (float64, float64) {
	asks, ok := orderBook["asks"].(map[string]interface{})
	if !ok {
		return 0, 0
	}

	bestPrice := 999999999.0
	bestVolume := 0.0

	for priceStr, volumeInterface := range asks {
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

		if price < bestPrice && volume > 0 {
			bestPrice = price
			bestVolume = volume
		}
	}

	if bestPrice == 999999999.0 {
		return 0, 0
	}
	return bestPrice, bestVolume
}

func (e *ArbitrageExecutor) getBestBid(orderBook map[string]interface{}) (float64, float64) {
	bids, ok := orderBook["bids"].(map[string]interface{})
	if !ok {
		return 0, 0
	}

	bestPrice := 0.0
	bestVolume := 0.0

	for priceStr, volumeInterface := range bids {
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

		if price > bestPrice && volume > 0 {
			bestPrice = price
			bestVolume = volume
		}
	}

	return bestPrice, bestVolume
}

func (e *ArbitrageExecutor) executeRealTimeOrder(opportunity RealTimeOpportunity) types.ExecutedOrder {
	executedOrder := types.ExecutedOrder{
		OrderNumber:    1,
		Currency:       opportunity.Currency,
		BuyMarket:      opportunity.BuyMarket,
		SellMarket:     opportunity.SellMarket,
		PlannedVolume:  opportunity.Volume,
		ExpectedProfit: opportunity.ExpectedMargin * opportunity.Volume,
		StartTime:      time.Now(),
	}

	log.Printf("   🚀 EXECUTING: %.0f %s", opportunity.Volume, opportunity.Currency)

	// Step 1: BUY immediately
	log.Printf("   🟢 BUY: %.0f %s on %s", opportunity.Volume, opportunity.Currency, opportunity.BuyMarket)

	buyOrder, err := e.client.CreateOrder(coindcx.OrderRequest{
		Side:          "buy",
		OrderType:     "market_order",
		Market:        opportunity.BuyMarket,
		TotalQuantity: opportunity.Volume,
	})

	if err != nil {
		executedOrder.ErrorMessage = fmt.Sprintf("buy failed: %v", err)
		executedOrder.EndTime = time.Now()
		return executedOrder
	}

	if len(buyOrder.Orders) == 0 {
		executedOrder.ErrorMessage = "no buy order returned"
		executedOrder.EndTime = time.Now()
		return executedOrder
	}

	buyOrderID := buyOrder.Orders[0].ID
	executedOrder.BuyOrderID = buyOrderID

	// Wait for buy fill
	buyFilled, err := e.waitForOrderFill(buyOrderID, 10)
	if err != nil || !buyFilled {
		executedOrder.ErrorMessage = "buy timeout"
		executedOrder.EndTime = time.Now()
		return executedOrder
	}

	// Get buy details
	filledBuy, err := e.client.GetOrderStatus(buyOrderID)
	if err != nil {
		executedOrder.ErrorMessage = "buy status error"
		executedOrder.EndTime = time.Now()
		return executedOrder
	}

	actualVolume := filledBuy.TotalQuantity - filledBuy.RemainingQuantity
	executedOrder.VolumeExecuted = actualVolume
	executedOrder.BuyPrice = filledBuy.AvgPrice

	log.Printf("   ✅ Bought: %.0f at ₹%.6f", actualVolume, filledBuy.AvgPrice)

	// Step 2: SELL immediately for arbitrage
	log.Printf("   🔴 SELL: %.0f %s on %s", actualVolume, opportunity.Currency, opportunity.SellMarket)

	sellOrder, err := e.client.CreateOrder(coindcx.OrderRequest{
		Side:          "sell",
		OrderType:     "market_order",
		Market:        opportunity.SellMarket,
		TotalQuantity: actualVolume,
	})

	if err == nil && len(sellOrder.Orders) > 0 {
		sellOrderID := sellOrder.Orders[0].ID
		executedOrder.SellOrderID = sellOrderID

		sellFilled, err := e.waitForOrderFill(sellOrderID, 10)
		if err == nil && sellFilled {
			filledSell, err := e.client.GetOrderStatus(sellOrderID)
			if err == nil {
				executedOrder.SellPrice = filledSell.AvgPrice

				// Calculate actual profit
				buyValue := actualVolume * filledBuy.AvgPrice
				sellValue := actualVolume * filledSell.AvgPrice
				fees := filledBuy.FeeAmount + filledSell.FeeAmount

				executedOrder.ActualProfit = sellValue - buyValue - fees
				executedOrder.ActualMarginPct = (executedOrder.ActualProfit / buyValue) * 100
				executedOrder.Success = true

				log.Printf("   💰 ARBITRAGE: sold at ₹%.6f, profit ₹%.2f (%.2f%%)",
					filledSell.AvgPrice, executedOrder.ActualProfit, executedOrder.ActualMarginPct)

				executedOrder.EndTime = time.Now()
				return executedOrder
			}
		}
	}

	// Step 3: Recovery to USDT if arbitrage failed
	log.Printf("   ⚠️ Arbitrage failed, recovering...")
	recovered := e.recoverToUSDT(opportunity.Currency, actualVolume)

	if recovered.Success {
		buyValue := actualVolume * filledBuy.AvgPrice
		sellValue := actualVolume * recovered.SellPrice
		fees := filledBuy.FeeAmount + recovered.FeeAmount

		executedOrder.ActualProfit = sellValue - buyValue - fees
		executedOrder.ActualMarginPct = (executedOrder.ActualProfit / buyValue) * 100
		executedOrder.SellPrice = recovered.SellPrice
		executedOrder.SellOrderID = recovered.OrderID
		executedOrder.Success = true

		log.Printf("   🔄 Recovered: ₹%.2f (%.2f%%)", executedOrder.ActualProfit, executedOrder.ActualMarginPct)
	} else {
		executedOrder.ErrorMessage = "recovery failed"
	}

	executedOrder.EndTime = time.Now()
	return executedOrder
}

type RecoveryResult struct {
	Success   bool
	SellPrice float64
	FeeAmount float64
	OrderID   string
}

func (e *ArbitrageExecutor) recoverToUSDT(currency string, volume float64) RecoveryResult {
	market := fmt.Sprintf("%sUSDT", currency)

	sellOrder, err := e.client.CreateOrder(coindcx.OrderRequest{
		Side:          "sell",
		OrderType:     "market_order",
		Market:        market,
		TotalQuantity: volume,
	})

	if err != nil || len(sellOrder.Orders) == 0 {
		return RecoveryResult{Success: false}
	}

	orderID := sellOrder.Orders[0].ID
	filled, err := e.waitForOrderFill(orderID, 15)
	if err != nil || !filled {
		return RecoveryResult{Success: false}
	}

	finalOrder, err := e.client.GetOrderStatus(orderID)
	if err != nil {
		return RecoveryResult{Success: false}
	}

	return RecoveryResult{
		Success:   true,
		SellPrice: finalOrder.AvgPrice,
		FeeAmount: finalOrder.FeeAmount,
		OrderID:   orderID,
	}
}

func (e *ArbitrageExecutor) waitForOrderFill(orderID string, timeoutSeconds int) (bool, error) {
	timeout := time.After(time.Duration(timeoutSeconds) * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return false, fmt.Errorf("timeout")
		case <-ticker.C:
			order, err := e.client.GetOrderStatus(orderID)
			if err != nil {
				continue
			}

			switch order.Status {
			case "filled":
				return true, nil
			case "cancelled", "rejected":
				return false, fmt.Errorf("order %s", order.Status)
			default:
				continue
			}
		}
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (e *ArbitrageExecutor) DisplayResults(result *types.ExecutionResult) {
	fmt.Printf("\n📊 EXECUTION RESULTS:\n")
	fmt.Printf("====================\n")
	fmt.Printf("📊 Total Orders: %d\n", len(result.Orders))
	fmt.Printf("💰 Total Investment: $%.2f\n", result.TotalInvestment)
	fmt.Printf("💵 Total Profit: ₹%.2f\n", result.TotalProfit)
	fmt.Printf("📈 Success Rate: %.1f%%\n", e.calculateSuccessRate(result))
	fmt.Printf("⏱️ Total Time: %v\n", result.EndTime.Sub(result.StartTime))

	if len(result.Orders) > 0 {
		fmt.Printf("\n📋 Order Details:\n")
		for _, order := range result.Orders {
			status := "✅"
			if !order.Success {
				status = "❌"
			}
			fmt.Printf("   %s %s: %.0f tokens, ₹%.2f profit (%.2f%%)\n",
				status, order.Currency, order.VolumeExecuted,
				order.ActualProfit, order.ActualMarginPct)
		}
	}
}

func (e *ArbitrageExecutor) calculateSuccessRate(result *types.ExecutionResult) float64 {
	if len(result.Orders) == 0 {
		return 0.0
	}

	successful := 0
	for _, order := range result.Orders {
		if order.Success {
			successful++
		}
	}

	return (float64(successful) / float64(len(result.Orders))) * 100
}

func (e *ArbitrageExecutor) SaveExecutionLog(result *types.ExecutionResult, filename string) error {
	return utils.SaveJSON(result, filename)
}
