package opportunity

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/b-thark/cdcx-api/pkg/exchange"
	"github.com/b-thark/cdcx-api/pkg/market"
	"github.com/b-thark/cdcx-api/pkg/types"
	"github.com/b-thark/cdcx-api/pkg/utils"
)

type Detector struct {
	fetcher     *market.Fetcher
	rateManager *exchange.RateManager
	config      *types.Config
}

func NewDetector(config *types.Config) *Detector {
	return &Detector{
		fetcher:     market.NewFetcher(),
		rateManager: exchange.NewRateManager(config),
		config:      config,
	}
}

func (d *Detector) FindOpportunities(pairs map[string]types.ArbitragePairs) ([]types.ArbitrageOpportunity, error) {
	log.Println("🔍 Analyzing arbitrage opportunities...")

	opportunities := []types.ArbitrageOpportunity{}
	totalCurrencies := 0
	checkedCurrencies := 0

	for currency, pairGroup := range pairs {
		totalCurrencies++
		if len(pairGroup.Pairs) < 2 {
			continue
		}

		log.Printf("📊 Analyzing %s (%d pairs)...", currency, len(pairGroup.Pairs))

		currencyOpps, err := d.analyzeCurrency(currency, pairGroup.Pairs)
		if err != nil {
			log.Printf("❌ %s: %v", currency, err)
			continue
		}

		hasViable := false
		for _, opp := range currencyOpps {
			if opp.Viable {
				hasViable = true
				break
			}
		}

		if hasViable {
			checkedCurrencies++
		}

		opportunities = append(opportunities, currencyOpps...)
	}

	// Save rate cache
	d.rateManager.SaveCache()

	log.Printf("✅ Analysis complete: %d total currencies, %d with viable opportunities",
		totalCurrencies, checkedCurrencies)

	return opportunities, nil
}

func (d *Detector) analyzeCurrency(currency string, pairs []types.PairInfo) ([]types.ArbitrageOpportunity, error) {
	// Get current prices for all pairs
	pairPrices := make(map[string]PriceInfo)

	for _, pair := range pairs {
		priceInfo, err := d.getPriceInfo(pair)
		if err != nil {
			log.Printf("   ⚠️ %s: %v", pair.Symbol, err)
			continue
		}

		// Check liquidity
		bidLiquidityINR := priceInfo.BidVolume * priceInfo.BestBidINR
		askLiquidityINR := priceInfo.AskVolume * priceInfo.BestAskINR

		if bidLiquidityINR < d.config.MinLiquidity || askLiquidityINR < d.config.MinLiquidity {
			log.Printf("   📉 %s: Low liquidity (₹%.2f bid, ₹%.2f ask)",
				pair.Symbol, bidLiquidityINR, askLiquidityINR)
			continue
		}

		priceInfo.HasLiquidity = true
		pairPrices[pair.Symbol] = priceInfo
	}

	if len(pairPrices) < 2 {
		return nil, fmt.Errorf("insufficient liquid pairs")
	}

	// Find arbitrage opportunities between all pair combinations
	opportunities := []types.ArbitrageOpportunity{}

	for buySymbol, buyPrice := range pairPrices {
		for sellSymbol, sellPrice := range pairPrices {
			if buySymbol == sellSymbol || !buyPrice.HasLiquidity || !sellPrice.HasLiquidity {
				continue
			}

			opp := d.calculateArbitrage(currency, buyPrice, sellPrice)
			if opp.NetMarginPct >= d.config.MinNetMargin {
				opp.Viable = true
				log.Printf("   🎯 VIABLE: %s → %s (%.2f%% net margin)",
					buySymbol, sellSymbol, opp.NetMarginPct)
			} else {
				log.Printf("   ❌ %s → %s: %.2f%% margin (below %.1f%% threshold)",
					buySymbol, sellSymbol, opp.NetMarginPct, d.config.MinNetMargin)
			}

			opportunities = append(opportunities, opp)
		}
	}

	return opportunities, nil
}

type PriceInfo struct {
	Pair         types.PairInfo
	BestBid      float64
	BestAsk      float64
	BidVolume    float64
	AskVolume    float64
	BestBidINR   float64
	BestAskINR   float64
	HasLiquidity bool
}

func (d *Detector) getPriceInfo(pair types.PairInfo) (PriceInfo, error) {
	orderBook, err := d.fetcher.GetOrderBook(pair.Pair)
	if err != nil {
		return PriceInfo{}, err
	}

	priceInfo := PriceInfo{Pair: pair}

	// Parse bids (buy orders)
	if bids, ok := orderBook["bids"].(map[string]interface{}); ok {
		for priceStr, volumeInterface := range bids {
			price, _ := strconv.ParseFloat(priceStr, 64)
			var volume float64
			switch v := volumeInterface.(type) {
			case string:
				volume, _ = strconv.ParseFloat(v, 64)
			case float64:
				volume = v
			}

			if price > priceInfo.BestBid {
				priceInfo.BestBid = price
				priceInfo.BidVolume = volume
			}
		}
	}

	// Parse asks (sell orders)
	priceInfo.BestAsk = 999999999.0
	if asks, ok := orderBook["asks"].(map[string]interface{}); ok {
		for priceStr, volumeInterface := range asks {
			price, _ := strconv.ParseFloat(priceStr, 64)
			var volume float64
			switch v := volumeInterface.(type) {
			case string:
				volume, _ = strconv.ParseFloat(v, 64)
			case float64:
				volume = v
			}

			if price < priceInfo.BestAsk {
				priceInfo.BestAsk = price
				priceInfo.AskVolume = volume
			}
		}
	}

	// Convert to INR
	if priceInfo.BestBid > 0 {
		priceInfo.BestBidINR, _ = d.rateManager.ConvertToINR(priceInfo.BestBid, pair.BaseCurrency)
	}
	if priceInfo.BestAsk < 999999999.0 {
		priceInfo.BestAskINR, _ = d.rateManager.ConvertToINR(priceInfo.BestAsk, pair.BaseCurrency)
	}

	return priceInfo, nil
}

func (d *Detector) calculateArbitrage(currency string, buyPrice, sellPrice PriceInfo) types.ArbitrageOpportunity {
	// Calculate margins in INR terms
	grossMargin := sellPrice.BestBidINR - buyPrice.BestAskINR
	grossMarginPct := (grossMargin / buyPrice.BestAskINR) * 100

	// Estimate fees
	estimatedFees := (buyPrice.BestAskINR + sellPrice.BestBidINR) * d.config.FeeRate

	// Calculate net margins
	netMargin := grossMargin - estimatedFees
	netMarginPct := (netMargin / buyPrice.BestAskINR) * 100

	return types.ArbitrageOpportunity{
		TargetCurrency: currency,
		BuyMarket: struct {
			Symbol       string `json:"symbol"`
			Pair         string `json:"pair"`
			BaseCurrency string `json:"base_currency"`
		}{
			Symbol:       buyPrice.Pair.Symbol,
			Pair:         buyPrice.Pair.Pair,
			BaseCurrency: buyPrice.Pair.BaseCurrency,
		},
		SellMarket: struct {
			Symbol       string `json:"symbol"`
			Pair         string `json:"pair"`
			BaseCurrency string `json:"base_currency"`
		}{
			Symbol:       sellPrice.Pair.Symbol,
			Pair:         sellPrice.Pair.Pair,
			BaseCurrency: sellPrice.Pair.BaseCurrency,
		},
		BuyPriceINR:    buyPrice.BestAskINR,
		SellPriceINR:   sellPrice.BestBidINR,
		GrossMargin:    grossMargin,
		GrossMarginPct: grossMarginPct,
		EstimatedFees:  estimatedFees,
		NetMargin:      netMargin,
		NetMarginPct:   netMarginPct,
		Viable:         false, // Set by caller
		Timestamp:      time.Now(),
	}
}

func (d *Detector) SaveOpportunities(opportunities []types.ArbitrageOpportunity, filename string) error {
	return utils.SaveJSON(opportunities, filename)
}

func (d *Detector) LoadOpportunities(filename string) ([]types.ArbitrageOpportunity, error) {
	var opportunities []types.ArbitrageOpportunity
	err := utils.LoadJSON(filename, &opportunities)
	return opportunities, err
}

func (d *Detector) DisplayResults(opportunities []types.ArbitrageOpportunity) {
	fmt.Printf("\n🎯 ARBITRAGE OPPORTUNITY ANALYSIS RESULTS\n")
	fmt.Printf("========================================\n")

	// Filter viable opportunities
	viableOpps := []types.ArbitrageOpportunity{}
	for _, opp := range opportunities {
		if opp.Viable {
			viableOpps = append(viableOpps, opp)
		}
	}

	fmt.Printf("💰 Total opportunities found: %d\n", len(opportunities))
	fmt.Printf("✅ Viable opportunities: %d\n", len(viableOpps))

	if len(viableOpps) == 0 {
		fmt.Printf("\n❌ No viable arbitrage opportunities found with %.1f%%+ net margin\n", d.config.MinNetMargin)
		return
	}

	// Sort opportunities by net margin percentage (highest first)
	sort.Slice(viableOpps, func(i, j int) bool {
		return viableOpps[i].NetMarginPct > viableOpps[j].NetMarginPct
	})

	fmt.Printf("\n🔥 VIABLE ARBITRAGE OPPORTUNITIES:\n")
	fmt.Printf("================================\n")

	// Group by target currency for better display
	currencyOpps := make(map[string][]types.ArbitrageOpportunity)
	for _, opp := range viableOpps {
		currencyOpps[opp.TargetCurrency] = append(currencyOpps[opp.TargetCurrency], opp)
	}

	oppNum := 1
	for currency, opps := range currencyOpps {
		fmt.Printf("\n💎 %s (%d opportunities):\n", currency, len(opps))

		// Sort this currency's opportunities by margin
		sort.Slice(opps, func(i, j int) bool {
			return opps[i].NetMarginPct > opps[j].NetMarginPct
		})

		for _, opp := range opps {
			fmt.Printf("   %d. %s → %s\n", oppNum, opp.BuyMarket.Symbol, opp.SellMarket.Symbol)
			fmt.Printf("      🟢 BUY:  %s at ₹%.4f\n", opp.BuyMarket.Symbol, opp.BuyPriceINR)
			fmt.Printf("      🔴 SELL: %s at ₹%.4f\n", opp.SellMarket.Symbol, opp.SellPriceINR)
			fmt.Printf("      💵 Gross Margin: ₹%.4f (%.2f%%)\n", opp.GrossMargin, opp.GrossMarginPct)
			fmt.Printf("      💸 Est. Fees: ₹%.4f (%.1f%% buffer)\n", opp.EstimatedFees, d.config.FeeRate*100)
			fmt.Printf("      💰 Net Margin: ₹%.4f (%.2f%%)\n", opp.NetMargin, opp.NetMarginPct)
			fmt.Printf("      📊 Rating: %s\n", d.getRatingEmoji(opp.NetMarginPct))
			oppNum++
		}
	}

	// Display summary statistics
	d.displaySummaryStats(viableOpps)
}

func (d *Detector) getRatingEmoji(netMarginPct float64) string {
	if netMarginPct >= 5.0 {
		return "🔥 EXCELLENT"
	} else if netMarginPct >= 3.5 {
		return "⭐ VERY GOOD"
	} else if netMarginPct >= 2.5 {
		return "✅ GOOD"
	} else {
		return "⚠️ MARGINAL"
	}
}

func (d *Detector) displaySummaryStats(opportunities []types.ArbitrageOpportunity) {
	fmt.Printf("\n📈 SUMMARY STATISTICS:\n")
	fmt.Printf("=====================\n")

	if len(opportunities) == 0 {
		return
	}

	bestMargin := opportunities[0].NetMarginPct
	avgMargin := 0.0
	for _, opp := range opportunities {
		avgMargin += opp.NetMarginPct
	}
	avgMargin /= float64(len(opportunities))

	fmt.Printf("📊 Best Opportunity: %.2f%% net margin (%s)\n", bestMargin, opportunities[0].TargetCurrency)
	fmt.Printf("📊 Average Margin: %.2f%%\n", avgMargin)

	// Count by currency
	currencyCount := make(map[string]int)
	for _, opp := range opportunities {
		currencyCount[opp.TargetCurrency]++
	}

	fmt.Printf("📊 Opportunities by Currency:\n")
	for currency, count := range currencyCount {
		fmt.Printf("   %s: %d opportunities\n", currency, count)
	}
}
