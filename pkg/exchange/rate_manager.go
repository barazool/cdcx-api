package exchange

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/b-thark/cdcx-api/pkg/types"
)

type RateManager struct {
	cache  *types.ExchangeRateCache
	config *types.Config
	client *http.Client
}

func NewRateManager(config *types.Config) *RateManager {
	rm := &RateManager{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
	rm.loadCache()
	return rm
}

func (rm *RateManager) loadCache() {
	rm.cache = &types.ExchangeRateCache{
		Rates:       make(map[string]types.ExchangeRate),
		LastUpdated: time.Now(),
	}

	data, err := os.ReadFile(rm.config.RateCacheFile)
	if err != nil {
		return // Cache file doesn't exist yet
	}

	json.Unmarshal(data, rm.cache)
}

func (rm *RateManager) SaveCache() error {
	rm.cache.LastUpdated = time.Now()
	data, err := json.MarshalIndent(rm.cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rm.config.RateCacheFile, data, 0644)
}

func (rm *RateManager) ConvertToINR(price float64, fromCurrency string) (float64, error) {
	if fromCurrency == "INR" {
		return price, nil
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s_INR", fromCurrency)
	if rate, exists := rm.cache.Rates[cacheKey]; exists {
		if time.Since(rate.Timestamp) < rm.config.CacheDuration {
			return price * rate.Rate, nil
		}
	}

	// Fetch new rate
	rate, err := rm.fetchExchangeRate(fromCurrency, "INR")
	if err != nil {
		return 0, err
	}

	// Update cache
	rm.cache.Rates[cacheKey] = rate
	return price * rate.Rate, nil
}

func (rm *RateManager) fetchExchangeRate(fromCurrency, toCurrency string) (types.ExchangeRate, error) {
	pair := fmt.Sprintf("%s%s", fromCurrency, toCurrency)
	url := "https://api.coindcx.com/exchange/ticker"

	resp, err := rm.client.Get(url)
	if err != nil {
		return types.ExchangeRate{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.ExchangeRate{}, err
	}

	var tickers []map[string]interface{}
	if err := json.Unmarshal(body, &tickers); err != nil {
		return types.ExchangeRate{}, err
	}

	for _, ticker := range tickers {
		if market, ok := ticker["market"].(string); ok && market == pair {
			if lastPriceStr, ok := ticker["last_price"].(string); ok {
				rate, err := strconv.ParseFloat(lastPriceStr, 64)
				if err == nil {
					return types.ExchangeRate{
						FromCurrency: fromCurrency,
						ToCurrency:   toCurrency,
						Rate:         rate,
						Timestamp:    time.Now(),
						Source:       "ticker",
					}, nil
				}
			}
		}
	}

	return types.ExchangeRate{}, fmt.Errorf("exchange rate not found for %s/%s", fromCurrency, toCurrency)
}
