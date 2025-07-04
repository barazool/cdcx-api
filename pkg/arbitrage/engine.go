package arbitrage

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/coindcx"
	"github.com/b-thark/cdcx-api/pkg/exchange"
	"github.com/b-thark/cdcx-api/pkg/market"
	"github.com/b-thark/cdcx-api/pkg/types"
	"github.com/b-thark/cdcx-api/pkg/utils"
)

type Engine struct {
	client      *coindcx.Client
	config      *types.ExecutionConfig
	apiConfig   *config.Config
	fetcher     *market.Fetcher
	rateManager *exchange.RateManager
	startTime   time.Time
}

func NewEngine(apiConfig *config.Config, execConfig *types.ExecutionConfig) *Engine {
	tradingConfig := types.DefaultConfig()
	return &Engine{
		client:      coindcx.NewClient(apiConfig.APIKey, apiConfig.APISecret),
		config:      execConfig,
		apiConfig:   apiConfig,
		fetcher:     market.NewFetcher(),
		rateManager: exchange.NewRateManager(tradingConfig),
		startTime:   time.Now(),
	}
}

func (e *Engine) LoadOpportunities(filename string) ([]types.ArbitrageOpportunity, error) {
	var opportunities []types.ArbitrageOpportunity
	err := utils.LoadJSON(filename, &opportunities)
	return opportunities, err
}

func (e *Engine) CheckAccountReadiness() (bool, error) {
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

func (e *Engine) DisplayExecutionPlan(opportunities []types.ArbitrageOpportunity) {
	viableCount := 0
	for _, opp := range opportunities {
		if opp.Viable {
			viableCount++
		}
	}

	fmt.Printf("🎯 Found %d viable opportunities for real-time analysis\n", viableCount)
	fmt.Printf("   💰 Max Position: $%.2f USDT\n", e.config.MaxPositionUSDT)
	fmt.Printf("   🛑 Stop Loss: %.1f%%\n", e.config.StopLossPct)
	fmt.Printf("   🔍 Mode: Real-time depth analysis + immediate execution\n")
}

type RealTimeOpportunity struct {
	Currency             string
	BuyMarket            string
	SellMarket           string
	BuyPrice             float64
	SellPrice            float64
	Volume               float64
	ExpectedMargin       float64
	MarginPct            float64
	Viable               bool
	Reason               string
	DepthAnalysis        types.QuickDepthResult
	MaxProfitableOrders  int
	TotalEstimatedProfit float64
}

func (e *Engine) Execute(opportunities []types.ArbitrageOpportunity) (*types.ExecutionResult, error) {
	result := &types.ExecutionResult{
		StartTime:  time.Now(),
		Timestamp:  time.Now(),
		Successful: false,
		Orders:     []types.ExecutedOrder{},
		Config:     *e.config,
	}

	totalProfit := 0.0
	totalInvestment := 0.0
	processedCount := 0

	// Filter and sort viable opportunities
	viableOpps := []types.ArbitrageOpportunity{}
	for _, opp := range opportunities {
		if opp.Viable && strings.Contains(opp.BuyMarket.Symbol, "USDT") {
			viableOpps = append(viableOpps, opp)
		}
	}

	// Sort by expected margin
	sort.Slice(viableOpps, func(i, j int) bool {
		return viableOpps[i].NetMarginPct > viableOpps[j].NetMarginPct
	})

	// fmt.Println("\n🔄 LIVE ARBITRAGE EXECUTION:")
	// fmt.Println("============================")

	for _, opp := range viableOpps {
		processedCount++
		// log.Printf("\n📊 [%d/%d] Processing %s (%s → %s)",
		// 	processedCount, len(viableOpps), opp.TargetCurrency,
		// 	opp.BuyMarket.Symbol, opp.SellMarket.Symbol)

		// Real-time depth analysis + validation
		liveOpp := e.analyzeAndValidateRealTime(opp)

		if !liveOpp.Viable {
			log.Printf("❌ %s: %s", opp.TargetCurrency, liveOpp.Reason)
			continue
		}

		// log.Printf("✅ %s: %.2f%% margin, %d profitable orders - EXECUTING",
		// 	opp.TargetCurrency, liveOpp.MarginPct, liveOpp.MaxProfitableOrders)

		// Execute immediately while conditions are good
		executedOrder := e.executeRealTimeOrder(liveOpp)
		result.Orders = append(result.Orders, executedOrder)

		if executedOrder.Success {
			totalProfit += executedOrder.ActualProfit
			totalInvestment += (executedOrder.VolumeExecuted * executedOrder.BuyPrice) / 83.0
			log.Printf("💰 %s SUCCESS: ₹%.2f profit", opp.TargetCurrency, executedOrder.ActualProfit)
		}

		// Check limits
		if totalInvestment >= e.config.MaxPositionUSDT {
			log.Printf("💰 Position limit reached: $%.2f", e.config.MaxPositionUSDT)
			break
		}

		// Small delay between executions
		time.Sleep(time.Duration(e.config.DelayBetweenOrders) * time.Millisecond)
	}

	result.EndTime = time.Now()
	result.TotalProfit = totalProfit
	result.TotalInvestment = totalInvestment
	result.Successful = totalProfit > 0

	return result, nil
}

func (e *Engine) analyzeAndValidateRealTime(opp types.ArbitrageOpportunity) RealTimeOpportunity {
	liveOpp := RealTimeOpportunity{
		Currency:   opp.TargetCurrency,
		BuyMarket:  opp.BuyMarket.Symbol,
		SellMarket: opp.SellMarket.Symbol,
		Viable:     false,
	}

	// Step 1: Get fresh order book data
	buyOrderBook, err := e.fetcher.GetOrderBook(opp.BuyMarket.Pair)
	if err != nil {
		liveOpp.Reason = fmt.Sprintf("buy market data error: %v", err)
		return liveOpp
	}

	sellOrderBook, err := e.fetcher.GetOrderBook(opp.SellMarket.Pair)
	if err != nil {
		liveOpp.Reason = fmt.Sprintf("sell market data error: %v", err)
		return liveOpp
	}

	// Step 2: Perform real-time depth analysis
	depthResult := e.performQuickDepthAnalysis(opp.TargetCurrency, buyOrderBook, sellOrderBook)
	liveOpp.DepthAnalysis = depthResult

	if depthResult.MaxProfitableOrders == 0 {
		liveOpp.Reason = "no profitable depth found"
		return liveOpp
	}

	// Step 3: Validate current best prices
	buyPrice, buyVolume := e.getBestAsk(buyOrderBook)
	sellPrice, sellVolume := e.getBestBid(sellOrderBook)

	if buyPrice == 0 || sellPrice == 0 {
		liveOpp.Reason = "no valid prices available"
		return liveOpp
	}

	if sellPrice <= buyPrice {
		liveOpp.Reason = fmt.Sprintf("no arbitrage: sell ₹%.6f <= buy ₹%.6f", sellPrice, buyPrice)
		return liveOpp
	}

	// Step 4: Calculate current margins
	grossMargin := sellPrice - buyPrice
	estimatedFees := (buyPrice + sellPrice) * 0.01 // 1% each side
	netMargin := grossMargin - estimatedFees
	netMarginPct := (netMargin / buyPrice) * 100

	liveOpp.BuyPrice = buyPrice
	liveOpp.SellPrice = sellPrice
	liveOpp.ExpectedMargin = netMargin
	liveOpp.MarginPct = netMarginPct
	liveOpp.MaxProfitableOrders = depthResult.MaxProfitableOrders
	liveOpp.TotalEstimatedProfit = depthResult.TotalEstimatedProfit

	// Step 5: Check volume and margin thresholds
	minVolume := 1000.0
	maxVolume := min(buyVolume, sellVolume)

	if maxVolume < minVolume {
		liveOpp.Reason = fmt.Sprintf("insufficient volume: %.0f < %.0f", maxVolume, minVolume)
		return liveOpp
	}

	if netMarginPct < e.config.StopLossPct {
		liveOpp.Reason = fmt.Sprintf("margin too low: %.2f%% < %.1f%%", netMarginPct, e.config.StopLossPct)
		return liveOpp
	}

	// Opportunity is viable
	liveOpp.Volume = min(maxVolume, 5000.0) // Cap at reasonable volume
	liveOpp.Viable = true
	liveOpp.Reason = "profitable arbitrage with sufficient depth"

	log.Printf("   💡 Live prices: Buy ₹%.6f, Sell ₹%.6f", buyPrice, sellPrice)
	log.Printf("   📊 Net margin: ₹%.6f (%.2f%%), Depth: %d orders", netMargin, netMarginPct, depthResult.MaxProfitableOrders)

	return liveOpp
}

func (e *Engine) performQuickDepthAnalysis(currency string, buyOrderBook, sellOrderBook map[string]interface{}) types.QuickDepthResult {
	result := types.QuickDepthResult{
		Currency:             currency,
		MaxProfitableOrders:  0,
		TotalEstimatedProfit: 0,
		BottleneckSide:       "none",
	}

	// Parse order book levels (top 5 levels for speed)
	buyLevels := e.parseOrderBookLevels(buyOrderBook, "asks", 5)
	sellLevels := e.parseOrderBookLevels(sellOrderBook, "bids", 5)

	if len(buyLevels) == 0 || len(sellLevels) == 0 {
		return result
	}

	// Quick simulation
	buyIdx, sellIdx := 0, 0
	orderCount := 0
	totalProfit := 0.0

	for buyIdx < len(buyLevels) && sellIdx < len(sellLevels) && orderCount < 5 {
		buyLevel := buyLevels[buyIdx]
		sellLevel := sellLevels[sellIdx]

		// Calculate tradeable volume
		volume := min(buyLevel.Volume, sellLevel.Volume)
		if volume < 100 { // Skip tiny orders
			break
		}

		// Calculate margins
		grossMargin := sellLevel.Price - buyLevel.Price
		if grossMargin <= 0 {
			break
		}

		tradeValue := volume * buyLevel.Price
		fees := tradeValue * 0.02 // 2% total fees
		netProfit := (grossMargin * volume) - fees
		netMarginPct := (netProfit / tradeValue) * 100

		if netMarginPct < e.config.StopLossPct {
			break
		}

		orderCount++
		totalProfit += netProfit

		// Move to next levels
		if buyLevel.Volume <= sellLevel.Volume {
			buyIdx++
		}
		if sellLevel.Volume <= buyLevel.Volume {
			sellIdx++
		}
	}

	result.MaxProfitableOrders = orderCount
	result.TotalEstimatedProfit = totalProfit

	if buyIdx >= len(buyLevels) {
		result.BottleneckSide = "buy"
	} else if sellIdx >= len(sellLevels) {
		result.BottleneckSide = "sell"
	}

	return result
}

func (e *Engine) parseOrderBookLevels(orderBook map[string]interface{}, side string, maxLevels int) []types.OrderLevel {
	levels := []types.OrderLevel{}

	orders, ok := orderBook[side].(map[string]interface{})
	if !ok {
		return levels
	}

	type priceLevel struct {
		price  float64
		volume float64
	}

	priceLevels := []priceLevel{}

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
			priceLevels = append(priceLevels, priceLevel{price: price, volume: volume})
		}
	}

	// Sort levels
	if side == "bids" {
		sort.Slice(priceLevels, func(i, j int) bool {
			return priceLevels[i].price > priceLevels[j].price
		})
	} else {
		sort.Slice(priceLevels, func(i, j int) bool {
			return priceLevels[i].price < priceLevels[j].price
		})
	}

	// Convert to OrderLevel and limit count
	maxCount := minInt(len(priceLevels), maxLevels)
	for i := 0; i < maxCount; i++ {
		level := priceLevels[i]
		levels = append(levels, types.OrderLevel{
			Price:  level.price,
			Volume: level.volume,
		})
	}

	return levels
}

func (e *Engine) getBestAsk(orderBook map[string]interface{}) (float64, float64) {
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

func (e *Engine) getBestBid(orderBook map[string]interface{}) (float64, float64) {
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

func (e *Engine) executeRealTimeOrder(opportunity RealTimeOpportunity) types.ExecutedOrder {
	executedOrder := types.ExecutedOrder{
		OrderNumber:    1,
		Currency:       opportunity.Currency,
		BuyMarket:      opportunity.BuyMarket,
		SellMarket:     opportunity.SellMarket,
		PlannedVolume:  opportunity.Volume,
		ExpectedProfit: opportunity.ExpectedMargin * opportunity.Volume,
		StartTime:      time.Now(),
	}

	// log.Printf("   🚀 EXECUTING: %.0f %s", opportunity.Volume, opportunity.Currency)

	// Step 1: BUY immediately
	// log.Printf("   🟢 BUY: %.0f %s on %s", opportunity.Volume, opportunity.Currency, opportunity.BuyMarket)

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
	buyFilled, err := e.waitForOrderFill(buyOrderID, e.config.OrderTimeoutSeconds)
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

	// log.Printf("   ✅ Bought: %.0f at ₹%.6f", actualVolume, filledBuy.AvgPrice)

	// Step 2: SELL immediately for arbitrage
	// log.Printf("   🔴 SELL: %.0f %s on %s", actualVolume, opportunity.Currency, opportunity.SellMarket)

	sellOrder, err := e.client.CreateOrder(coindcx.OrderRequest{
		Side:          "sell",
		OrderType:     "market_order",
		Market:        opportunity.SellMarket,
		TotalQuantity: actualVolume,
	})

	if err == nil && len(sellOrder.Orders) > 0 {
		sellOrderID := sellOrder.Orders[0].ID
		executedOrder.SellOrderID = sellOrderID

		sellFilled, err := e.waitForOrderFill(sellOrderID, e.config.OrderTimeoutSeconds)
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
				executedOrder.ExecutionTimeMs = executedOrder.EndTime.Sub(executedOrder.StartTime).Milliseconds()
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
	executedOrder.ExecutionTimeMs = executedOrder.EndTime.Sub(executedOrder.StartTime).Milliseconds()
	return executedOrder
}

type RecoveryResult struct {
	Success   bool
	SellPrice float64
	FeeAmount float64
	OrderID   string
}

func (e *Engine) recoverToUSDT(currency string, volume float64) RecoveryResult {
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

func (e *Engine) waitForOrderFill(orderID string, timeoutSeconds int) (bool, error) {
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (e *Engine) DisplayResults(result *types.ExecutionResult) {
	fmt.Printf("\n📊 LIVE ARBITRAGE RESULTS:\n")
	fmt.Printf("=========================\n")
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
			fmt.Printf("   %s %s: %.0f tokens, ₹%.2f profit (%.2f%%) in %dms\n",
				status, order.Currency, order.VolumeExecuted,
				order.ActualProfit, order.ActualMarginPct, order.ExecutionTimeMs)
		}
	}
}

func (e *Engine) calculateSuccessRate(result *types.ExecutionResult) float64 {
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

func (e *Engine) SaveExecutionLog(result *types.ExecutionResult, filename string) error {
	return utils.SaveJSON(result, filename)
}

func (e *Engine) AnalyzeAndValidateRealTime(opp types.ArbitrageOpportunity) RealTimeOpportunity {
	return e.analyzeAndValidateRealTime(opp)
}

// ExecuteRealTimeOrder - made public for use by live detector
func (e *Engine) ExecuteRealTimeOrder(opportunity RealTimeOpportunity) types.ExecutedOrder {
	return e.executeRealTimeOrder(opportunity)
}
