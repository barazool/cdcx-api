package arbitrage

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/b-thark/cdcx-api/pkg/coindcx"
)

// Detector handles arbitrage opportunity detection
type Detector struct {
	client  *coindcx.Client
	context TradingContext
}

// NewDetector creates a new arbitrage detector
func NewDetector(client *coindcx.Client) *Detector {
	// Initialize with Regular 1 fee structure (worst case)
	defaultFeeStructure := FeeStructure{
		Level:           "Regular 1",
		SpotINRFee:      SpotINRFeeRegular1,
		SpotC2CFee:      SpotC2CFeeRegular1,
		VolumeThreshold: 0,
	}

	context := TradingContext{
		UserVolume30Day: 0, // Assume new user
		CurrentFeeLevel: defaultFeeStructure,
		USDTBalance:     0,     // Will be fetched from API
		HasTDSThreshold: false, // Assume below threshold initially
	}

	return &Detector{
		client:  client,
		context: context,
	}
}

// UpdateContext updates the trading context with current user data
func (d *Detector) UpdateContext() error {
	// Get user balances
	balances, err := d.client.GetBalances()
	if err != nil {
		return fmt.Errorf("failed to get balances: %v", err)
	}

	// Find USDT balance
	for _, balance := range balances {
		if balance.Currency == "USDT" {
			d.context.USDTBalance = balance.Balance
			break
		}
	}

	return nil
}

// AnalyzeMarkets discovers all available arbitrage opportunities
func (d *Detector) AnalyzeMarkets() (*ArbitrageMatrix, error) {
	fmt.Println("🔍 Starting 2-step arbitrage analysis...")

	// Get all market details
	marketsDetails, err := d.client.GetMarketsDetails()
	if err != nil {
		return nil, fmt.Errorf("failed to get markets details: %v", err)
	}

	fmt.Printf("📊 Found %d total markets\n", len(marketsDetails))

	// Build arbitrage matrix
	matrix := &ArbitrageMatrix{
		USDTPairs:         []MarketPair{},
		TargetPairs:       make(map[string][]MarketPair),
		Opportunities:     []ArbitrageOpportunity{},
		TotalPairs:        len(marketsDetails),
		AnalysisTimestamp: time.Now().Unix(),
	}

	// Target currencies we're interested in
	targetCurrencies := []string{"INR", "BTC", "ETH"}

	// Initialize target pairs map
	for _, currency := range targetCurrencies {
		matrix.TargetPairs[currency] = []MarketPair{}
	}

	// Track coins that have USDT pairs
	coinMap := make(map[string]bool)

	for _, market := range marketsDetails {
		if market.Status != "active" {
			continue
		}

		marketPair := MarketPair{
			Pair:                market.Pair,
			BaseCurrency:        market.BaseCurrencyShortName,
			TargetCurrency:      market.TargetCurrencyShortName,
			Status:              market.Status,
			MinQuantity:         market.MinQuantity,
			MaxQuantity:         market.MaxQuantity,
			MinNotional:         market.MinNotional,
			AvailableOrderTypes: market.OrderTypes,
			IsActive:            market.Status == "active",
		}

		// If this is a USDT pair, add to USDT pairs and mark the coin
		if market.BaseCurrencyShortName == "USDT" {
			matrix.USDTPairs = append(matrix.USDTPairs, marketPair)
			coinMap[market.TargetCurrencyShortName] = true
		}

		// If this is a target currency pair, add to target pairs
		for _, targetCurrency := range targetCurrencies {
			if market.BaseCurrencyShortName == targetCurrency {
				matrix.TargetPairs[targetCurrency] = append(matrix.TargetPairs[targetCurrency], marketPair)
			}
		}
	}

	fmt.Printf("💰 Found %d USDT pairs\n", len(matrix.USDTPairs))
	for currency, pairs := range matrix.TargetPairs {
		fmt.Printf("🔄 Found %d %s pairs\n", len(pairs), currency)
	}

	// Find 2-step arbitrage opportunities
	opportunities := d.find2StepOpportunities(matrix, coinMap)
	matrix.Opportunities = opportunities
	matrix.TotalOpportunities = len(opportunities)

	return matrix, nil
}

// find2StepOpportunities finds all possible 2-step arbitrage paths
func (d *Detector) find2StepOpportunities(matrix *ArbitrageMatrix, coinMap map[string]bool) []ArbitrageOpportunity {
	var opportunities []ArbitrageOpportunity

	fmt.Println("\n🔎 Analyzing 2-step arbitrage opportunities (USDT → COIN → INR/BTC/ETH)...")

	// For each USDT pair, check if there's a corresponding target pair
	for _, usdtPair := range matrix.USDTPairs {
		coin := usdtPair.TargetCurrency

		// Check each target currency
		for targetCurrency, targetPairs := range matrix.TargetPairs {
			// Find if this coin trades with the target currency
			for _, targetPair := range targetPairs {
				if targetPair.TargetCurrency == coin {
					// Found a 2-step arbitrage opportunity!
					opportunity := ArbitrageOpportunity{
						SourcePair:     usdtPair.Pair,   // USDT → COIN
						TargetPair:     targetPair.Pair, // COIN → TARGET
						Coin:           coin,
						SourceCurrency: "USDT",
						TargetCurrency: targetCurrency,
						IsExecutable:   false, // Will be determined after price analysis
					}

					// Add basic trade info
					opportunity.MinInvestment = usdtPair.MinNotional
					opportunities = append(opportunities, opportunity)
				}
			}
		}
	}

	fmt.Printf("✅ Found %d potential 2-step arbitrage paths\n", len(opportunities))

	return opportunities
}

// AnalyzePrices fetches current prices and calculates profitability for opportunities
func (d *Detector) AnalyzePrices(opportunities []ArbitrageOpportunity) ([]ArbitrageOpportunity, error) {
	fmt.Println("\n💹 Analyzing prices for 2-step arbitrage opportunities...")

	var viableOpportunities []ArbitrageOpportunity

	for i, opp := range opportunities {
		fmt.Printf("🔍 Analyzing %s → %s (%d/%d)\n",
			opp.SourcePair, opp.TargetPair, i+1, len(opportunities))

		// Get order books for both steps
		sourceOrderBook, err := d.client.GetOrderBook(opp.SourcePair)
		if err != nil {
			fmt.Printf("❌ Failed to get orderbook for %s: %v\n", opp.SourcePair, err)
			continue
		}

		targetOrderBook, err := d.client.GetOrderBook(opp.TargetPair)
		if err != nil {
			fmt.Printf("❌ Failed to get orderbook for %s: %v\n", opp.TargetPair, err)
			continue
		}

		// Validate order books have data
		if len(sourceOrderBook.Asks) == 0 || len(sourceOrderBook.Bids) == 0 {
			fmt.Printf("❌ %s orderbook has no asks or bids\n", opp.SourcePair)
			continue
		}

		if len(targetOrderBook.Asks) == 0 || len(targetOrderBook.Bids) == 0 {
			fmt.Printf("❌ %s orderbook has no asks or bids\n", opp.TargetPair)
			continue
		}

		// Calculate profitability
		updatedOpp := d.calculateProfitability(opp, sourceOrderBook, targetOrderBook)

		// Only include if profitable and executable
		if updatedOpp.IsExecutable && updatedOpp.FinalProfit > 0 {
			viableOpportunities = append(viableOpportunities, updatedOpp)
		}

		// Add small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("\n🎯 Found %d viable opportunities out of %d analyzed\n",
		len(viableOpportunities), len(opportunities))

	return viableOpportunities, nil
}

// calculateProfitability calculates the profitability of a 2-step arbitrage opportunity
func (d *Detector) calculateProfitability(opp ArbitrageOpportunity, sourceOB, targetOB *coindcx.OrderBook) ArbitrageOpportunity {
	// Get best prices from order books
	sourceBuyPrice, sourceBuyVolume := d.getBestAskPrice(sourceOB)
	targetSellPrice, targetSellVolume := d.getBestBidPrice(targetOB)

	// Debug output for prices
	fmt.Printf("    📈 Prices: Buy %s at %.6f %s, Sell at %.6f %s\n",
		opp.Coin, sourceBuyPrice, opp.SourceCurrency, targetSellPrice, opp.TargetCurrency)

	if sourceBuyPrice == 0 || targetSellPrice == 0 {
		fmt.Printf("    ❌ Invalid prices (zero values)\n")
		return opp
	}

	// Update opportunity with price data
	opp.SourceBuyPrice = sourceBuyPrice
	opp.TargetSellPrice = targetSellPrice
	opp.SourceBuyVolume = sourceBuyVolume
	opp.TargetSellVolume = targetSellVolume
	opp.MaxTradeVolume = min(sourceBuyVolume, targetSellVolume)

	// Calculate costs and profits
	return d.calculateCostsAndProfits(opp)
}

// calculateCostsAndProfits calculates all costs, fees, taxes and final profit
func (d *Detector) calculateCostsAndProfits(opp ArbitrageOpportunity) ArbitrageOpportunity {
	// Assume we trade 1 unit of the coin for calculation
	tradeAmount := 1.0

	// Cost to buy 1 coin with USDT (in USDT)
	buyCostUSDT := opp.SourceBuyPrice * tradeAmount

	// Revenue from selling 1 coin for target currency
	sellRevenue := opp.TargetSellPrice * tradeAmount

	// Convert to INR for consistent comparison (assuming 1 USDT ≈ 85 INR)
	usdtToINRRate := 85.0
	buyCostINR := buyCostUSDT * usdtToINRRate

	var sellRevenueINR float64
	if opp.TargetCurrency == "INR" {
		sellRevenueINR = sellRevenue
	} else if opp.TargetCurrency == "BTC" {
		// Convert BTC to INR (approximate current rate: 1 BTC ≈ 92,00,000 INR)
		btcToINRRate := 9200000.0
		sellRevenueINR = sellRevenue * btcToINRRate
	} else if opp.TargetCurrency == "ETH" {
		// Convert ETH to INR (approximate current rate: 1 ETH ≈ 2,10,000 INR)
		ethToINRRate := 210000.0
		sellRevenueINR = sellRevenue * ethToINRRate
	} else {
		// For other currencies, skip for now
		opp.IsExecutable = false
		return opp
	}

	// Gross profit before any costs (in INR)
	opp.GrossProfit = sellRevenueINR - buyCostINR

	// Debug output
	fmt.Printf("    💰 %s: Buy %.4f USDT (₹%.2f) -> Sell %.6f %s (₹%.2f) = Gross ₹%.2f\n",
		opp.Coin, buyCostUSDT, buyCostINR, sellRevenue, opp.TargetCurrency, sellRevenueINR, opp.GrossProfit)

	// Calculate trading fees (in INR)
	buyFeeINR := buyCostINR * d.context.CurrentFeeLevel.SpotC2CFee // USDT -> COIN (C2C)

	var sellFeeINR float64
	if opp.TargetCurrency == "INR" {
		sellFeeINR = sellRevenueINR * d.context.CurrentFeeLevel.SpotINRFee // COIN -> INR (C2F)
	} else {
		sellFeeINR = sellRevenueINR * d.context.CurrentFeeLevel.SpotC2CFee // COIN -> CRYPTO (C2C)
	}

	opp.TradingFees = buyFeeINR + sellFeeINR

	// Calculate TDS (only applicable for INR conversions)
	if opp.TargetCurrency == "INR" && d.context.HasTDSThreshold {
		opp.TDSAmount = sellRevenueINR * TDSRate
	}

	// Net profit after fees and TDS
	opp.NetProfit = opp.GrossProfit - opp.TradingFees - opp.TDSAmount

	// Debug output for fees
	fmt.Printf("    📊 Fees: Buy ₹%.2f + Sell ₹%.2f + TDS ₹%.2f = Total ₹%.2f\n",
		buyFeeINR, sellFeeINR, opp.TDSAmount, opp.TradingFees+opp.TDSAmount)

	// Tax calculations (30% + 4% cess on net profit)
	if opp.NetProfit > 0 {
		opp.TaxableAmount = opp.NetProfit
		capitalGainsTax := opp.TaxableAmount * CapitalGainsTax
		cess := capitalGainsTax * CessRate
		opp.TaxLiability = capitalGainsTax + cess

		// Final profit after all taxes (can claim TDS against tax liability)
		actualTaxPayable := opp.TaxLiability - opp.TDSAmount
		if actualTaxPayable < 0 {
			actualTaxPayable = 0 // TDS covers all tax
		}
		opp.FinalProfit = opp.NetProfit - actualTaxPayable
	} else {
		opp.FinalProfit = opp.NetProfit // No tax on losses
	}

	// Calculate percentages (use INR buy cost for consistency)
	if buyCostINR > 0 {
		opp.ProfitPercent = (opp.NetProfit / buyCostINR) * 100
		opp.ROI = (opp.FinalProfit / buyCostINR) * 100
	}

	// Determine if executable
	hasMinProfit := opp.FinalProfit > 0
	hasMinROI := opp.ROI >= MinProfitThreshold*100
	hasLiquidity := opp.MaxTradeVolume >= 0.01
	hasBalance := buyCostUSDT <= d.context.USDTBalance || d.context.USDTBalance == 0

	opp.IsExecutable = hasMinProfit && hasMinROI && hasLiquidity && hasBalance

	// Debug output for executability
	fmt.Printf("    ⚡ Final: ₹%.2f profit (%.2f%% ROI) - ", opp.FinalProfit, opp.ROI)
	if opp.IsExecutable {
		fmt.Printf("✅ VIABLE\n")
	} else {
		fmt.Printf("❌ NOT VIABLE (")
		if !hasMinProfit {
			fmt.Printf("no profit, ")
		}
		if !hasMinROI {
			fmt.Printf("low ROI, ")
		}
		if !hasLiquidity {
			fmt.Printf("low liquidity, ")
		}
		if !hasBalance {
			fmt.Printf("insufficient balance, ")
		}
		fmt.Printf(")\n")
	}

	opp.MinInvestment = buyCostINR

	return opp
}

// getBestAskPrice gets the best ask (sell) price from order book
func (d *Detector) getBestAskPrice(orderBook *coindcx.OrderBook) (float64, float64) {
	var bestPrice, bestVolume float64

	for priceStr, volumeStr := range orderBook.Asks {
		price, err1 := strconv.ParseFloat(priceStr, 64)
		volume, err2 := strconv.ParseFloat(volumeStr, 64)

		if err1 != nil || err2 != nil {
			continue
		}

		if price <= 0 || volume <= 0 {
			continue
		}

		if bestPrice == 0 || price < bestPrice {
			bestPrice = price
			bestVolume = volume
		}
	}

	return bestPrice, bestVolume
}

// getBestBidPrice gets the best bid (buy) price from order book
func (d *Detector) getBestBidPrice(orderBook *coindcx.OrderBook) (float64, float64) {
	var bestPrice, bestVolume float64

	for priceStr, volumeStr := range orderBook.Bids {
		price, err1 := strconv.ParseFloat(priceStr, 64)
		volume, err2 := strconv.ParseFloat(volumeStr, 64)

		if err1 != nil || err2 != nil {
			continue
		}

		if price <= 0 || volume <= 0 {
			continue
		}

		if price > bestPrice {
			bestPrice = price
			bestVolume = volume
		}
	}

	return bestPrice, bestVolume
}

// min helper function
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// GetTopOpportunities returns the most profitable opportunities
func (d *Detector) GetTopOpportunities(opportunities []ArbitrageOpportunity, limit int) []ArbitrageOpportunity {
	if len(opportunities) == 0 {
		return opportunities
	}

	// Sort by ROI (descending)
	for i := 0; i < len(opportunities)-1; i++ {
		for j := i + 1; j < len(opportunities); j++ {
			if opportunities[i].ROI < opportunities[j].ROI {
				opportunities[i], opportunities[j] = opportunities[j], opportunities[i]
			}
		}
	}

	// Return top N opportunities
	if limit > len(opportunities) {
		limit = len(opportunities)
	}
	return opportunities[:limit]
}

// PrintOpportunityDetails prints detailed information about an opportunity
func (d *Detector) PrintOpportunityDetails(opp ArbitrageOpportunity) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("🎯 ARBITRAGE OPPORTUNITY: %s\n", opp.Coin)
	fmt.Printf(strings.Repeat("=", 60) + "\n")

	fmt.Printf("📊 Trading Path:\n")
	fmt.Printf("   1. Buy %s with %s at %.6f (%s)\n",
		opp.Coin, opp.SourceCurrency, opp.SourceBuyPrice, opp.SourcePair)
	fmt.Printf("   2. Sell %s for %s at %.6f (%s)\n",
		opp.Coin, opp.TargetCurrency, opp.TargetSellPrice, opp.TargetPair)

	fmt.Printf("\n💰 Profit Analysis (per unit):\n")
	fmt.Printf("   Gross Profit:     ₹%.4f\n", opp.GrossProfit)
	fmt.Printf("   Trading Fees:     ₹%.4f (%.2f%%)\n",
		opp.TradingFees, (opp.TradingFees/opp.MinInvestment)*100)

	if opp.TDSAmount > 0 {
		fmt.Printf("   TDS (1%%):        ₹%.4f\n", opp.TDSAmount)
	}

	fmt.Printf("   Net Profit:       ₹%.4f\n", opp.NetProfit)

	if opp.TaxLiability > 0 {
		fmt.Printf("   Tax Liability:    ₹%.4f (30%% + 4%% cess)\n", opp.TaxLiability)
	}

	fmt.Printf("   Final Profit:     ₹%.4f\n", opp.FinalProfit)
	fmt.Printf("   ROI:              %.2f%%\n", opp.ROI)

	fmt.Printf("\n📈 Volume & Liquidity:\n")
	fmt.Printf("   Source Volume:    %.4f %s\n", opp.SourceBuyVolume, opp.Coin)
	fmt.Printf("   Target Volume:    %.4f %s\n", opp.TargetSellVolume, opp.Coin)
	fmt.Printf("   Max Trade Volume: %.4f %s\n", opp.MaxTradeVolume, opp.Coin)
	fmt.Printf("   Min Investment:   ₹%.2f\n", opp.MinInvestment)

	fmt.Printf("\n⚡ Execution Status:\n")
	if opp.IsExecutable {
		fmt.Printf("   ✅ EXECUTABLE - This opportunity meets all criteria\n")
	} else {
		fmt.Printf("   ❌ NOT EXECUTABLE - ")
		if opp.ROI < MinProfitThreshold*100 {
			fmt.Printf("Profit below minimum threshold")
		} else if opp.MaxTradeVolume < MinTradeAmount {
			fmt.Printf("Insufficient liquidity")
		} else if opp.MinInvestment > d.context.USDTBalance {
			fmt.Printf("Insufficient USDT balance")
		} else {
			fmt.Printf("Other constraints not met")
		}
		fmt.Println()
	}

	fmt.Printf(strings.Repeat("=", 60) + "\n")
}
