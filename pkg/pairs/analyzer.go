package pairs

import (
	"fmt"
	"log"
	"time"

	"github.com/b-thark/cdcx-api/pkg/market"
	"github.com/b-thark/cdcx-api/pkg/types"
	"github.com/b-thark/cdcx-api/pkg/utils"
)

type Analyzer struct {
	fetcher *market.Fetcher
	config  *types.Config
}

func NewAnalyzer(config *types.Config) *Analyzer {
	return &Analyzer{
		fetcher: market.NewFetcher(),
		config:  config,
	}
}

func (a *Analyzer) ExtractArbitragePairs() (map[string]types.ArbitragePairs, error) {
	log.Println("🔍 Fetching market details...")

	markets, err := a.fetcher.GetMarketDetails()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch markets: %v", err)
	}

	log.Printf("✅ Found %d total markets", len(markets))

	// Group pairs by target currency
	allPairs := make(map[string][]types.PairInfo)

	for _, market := range markets {
		if market.Status != "active" {
			continue
		}

		targetCurrency := market.TargetCurrencyShortName

		pairInfo := types.PairInfo{
			Symbol:         market.Symbol,
			Pair:           market.Pair,
			BaseCurrency:   market.BaseCurrencyShortName,
			TargetCurrency: targetCurrency,
			MinQuantity:    market.MinQuantity,
			MinNotional:    market.MinNotional,
			Status:         market.Status,
		}

		allPairs[targetCurrency] = append(allPairs[targetCurrency], pairInfo)
	}

	// Filter pairs based on configuration
	arbitragePairs := make(map[string]types.ArbitragePairs)

	for targetCurrency, pairs := range allPairs {
		if len(pairs) < 2 {
			continue // Need at least 2 pairs for arbitrage
		}

		// Filter pairs by valid currencies if not enabling all pairs
		validPairs := []types.PairInfo{}
		for _, pair := range pairs {
			if a.isValidCurrency(pair.BaseCurrency) {
				validPairs = append(validPairs, pair)
			}
		}

		if len(validPairs) >= 2 {
			arbitragePairs[targetCurrency] = types.ArbitragePairs{
				TargetCurrency: targetCurrency,
				Pairs:          validPairs,
				LastUpdated:    time.Now(),
			}
		}
	}

	log.Printf("🎯 Found %d currencies with arbitrage potential", len(arbitragePairs))
	return arbitragePairs, nil
}

func (a *Analyzer) isValidCurrency(currency string) bool {
	if a.config.EnableAllPairs {
		return true
	}

	return utils.Contains(a.config.ValidCurrencies, currency)
}

func (a *Analyzer) SavePairs(pairs map[string]types.ArbitragePairs, filename string) error {
	return utils.SaveJSON(pairs, filename)
}

func (a *Analyzer) LoadPairs(filename string) (map[string]types.ArbitragePairs, error) {
	var pairs map[string]types.ArbitragePairs
	err := utils.LoadJSON(filename, &pairs)
	return pairs, err
}

func (a *Analyzer) DisplaySummary(pairs map[string]types.ArbitragePairs) {
	fmt.Printf("\n🎯 ARBITRAGE PAIRS ANALYSIS\n")
	fmt.Printf("==========================\n")
	fmt.Printf("📊 Total currencies with arbitrage potential: %d\n", len(pairs))

	if len(pairs) == 0 {
		fmt.Printf("❌ No arbitrage pairs found\n")
		return
	}

	// Count by base currency
	baseCurrencyCount := make(map[string]int)
	totalPairs := 0

	for currency, data := range pairs {
		fmt.Printf("\n💰 %s (%d pairs):\n", currency, len(data.Pairs))

		for _, pair := range data.Pairs {
			fmt.Printf("   📊 %s (%s) - Min: %.8f, Notional: %.8f\n",
				pair.Symbol, pair.BaseCurrency, pair.MinQuantity, pair.MinNotional)
			baseCurrencyCount[pair.BaseCurrency]++
			totalPairs++
		}
	}

	fmt.Printf("\n📈 SUMMARY:\n")
	fmt.Printf("   Total pairs: %d\n", totalPairs)
	fmt.Printf("   Base currency distribution:\n")
	for currency, count := range baseCurrencyCount {
		fmt.Printf("      %s: %d pairs\n", currency, count)
	}
}
