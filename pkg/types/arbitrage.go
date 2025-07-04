package types

import "time"

// Market and Pair Types
type MarketDetail struct {
	CoinDCXName             string   `json:"coindcx_name"`
	BaseCurrencyShortName   string   `json:"base_currency_short_name"`
	TargetCurrencyShortName string   `json:"target_currency_short_name"`
	TargetCurrencyName      string   `json:"target_currency_name"`
	BaseCurrencyName        string   `json:"base_currency_name"`
	MinQuantity             float64  `json:"min_quantity"`
	MaxQuantity             float64  `json:"max_quantity"`
	MinPrice                float64  `json:"min_price"`
	MaxPrice                float64  `json:"max_price"`
	MinNotional             float64  `json:"min_notional"`
	BaseCurrencyPrecision   int      `json:"base_currency_precision"`
	TargetCurrencyPrecision int      `json:"target_currency_precision"`
	Step                    float64  `json:"step"`
	OrderTypes              []string `json:"order_types"`
	Symbol                  string   `json:"symbol"`
	ECode                   string   `json:"ecode"`
	MaxLeverage             *float64 `json:"max_leverage"`
	MaxLeverageShort        *float64 `json:"max_leverage_short"`
	Pair                    string   `json:"pair"`
	Status                  string   `json:"status"`
}

type PairInfo struct {
	Symbol         string  `json:"symbol"`
	Pair           string  `json:"pair"`
	BaseCurrency   string  `json:"base_currency"`
	TargetCurrency string  `json:"target_currency"`
	MinQuantity    float64 `json:"min_quantity"`
	MinNotional    float64 `json:"min_notional"`
	Status         string  `json:"status"`
}

type ArbitragePairs struct {
	TargetCurrency string     `json:"target_currency"`
	Pairs          []PairInfo `json:"pairs"`
	LastUpdated    time.Time  `json:"last_updated"`
}

// Exchange Rate Types
type ExchangeRate struct {
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Rate         float64   `json:"rate"`
	Timestamp    time.Time `json:"timestamp"`
	Source       string    `json:"source"`
}

type ExchangeRateCache struct {
	Rates       map[string]ExchangeRate `json:"rates"`
	LastUpdated time.Time               `json:"last_updated"`
}

// Order Book Types
type OrderBookLevel struct {
	Price      float64 `json:"price"`
	Volume     float64 `json:"volume"`
	PriceINR   float64 `json:"price_inr"`
	Cumulative float64 `json:"cumulative"`
	VolumeINR  float64 `json:"volume_inr"`
}

type EnhancedOrderBook struct {
	Symbol         string           `json:"symbol"`
	Pair           string           `json:"pair"`
	BaseCurrency   string           `json:"base_currency"`
	BidLevels      []OrderBookLevel `json:"bid_levels"`
	AskLevels      []OrderBookLevel `json:"ask_levels"`
	BestBid        float64          `json:"best_bid"`
	BestAsk        float64          `json:"best_ask"`
	BestBidINR     float64          `json:"best_bid_inr"`
	BestAskINR     float64          `json:"best_ask_inr"`
	Spread         float64          `json:"spread"`
	SpreadPct      float64          `json:"spread_pct"`
	TotalBidVolume float64          `json:"total_bid_volume"`
	TotalAskVolume float64          `json:"total_ask_volume"`
	Timestamp      time.Time        `json:"timestamp"`
}

// Arbitrage Opportunity Types
type ArbitrageOpportunity struct {
	TargetCurrency string `json:"target_currency"`
	BuyMarket      struct {
		Symbol       string `json:"symbol"`
		Pair         string `json:"pair"`
		BaseCurrency string `json:"base_currency"`
	} `json:"buy_market"`
	SellMarket struct {
		Symbol       string `json:"symbol"`
		Pair         string `json:"pair"`
		BaseCurrency string `json:"base_currency"`
	} `json:"sell_market"`
	BuyPriceINR    float64   `json:"buy_price_inr"`
	SellPriceINR   float64   `json:"sell_price_inr"`
	GrossMargin    float64   `json:"gross_margin"`
	GrossMarginPct float64   `json:"gross_margin_pct"`
	EstimatedFees  float64   `json:"estimated_fees"`
	NetMargin      float64   `json:"net_margin"`
	NetMarginPct   float64   `json:"net_margin_pct"`
	Viable         bool      `json:"viable"`
	Timestamp      time.Time `json:"timestamp"`
}

// Quick Depth Analysis Types (for real-time processing)
type OrderLevel struct {
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
}

type QuickDepthResult struct {
	Currency             string  `json:"currency"`
	MaxProfitableOrders  int     `json:"max_profitable_orders"`
	TotalEstimatedProfit float64 `json:"total_estimated_profit"`
	BottleneckSide       string  `json:"bottleneck_side"`
}

// Legacy Depth Analysis Types (for backwards compatibility)
type OrderSimulation struct {
	OrderNumber    int     `json:"order_number"`
	BuyPrice       float64 `json:"buy_price"`
	SellPrice      float64 `json:"sell_price"`
	Volume         float64 `json:"volume"`
	VolumeINR      float64 `json:"volume_inr"`
	GrossMargin    float64 `json:"gross_margin"`
	GrossMarginPct float64 `json:"gross_margin_pct"`
	EstimatedFees  float64 `json:"estimated_fees"`
	NetMargin      float64 `json:"net_margin"`
	NetMarginPct   float64 `json:"net_margin_pct"`
	Profitable     bool    `json:"profitable"`
	Cumulative     struct {
		Volume    float64 `json:"volume"`
		VolumeINR float64 `json:"volume_inr"`
		NetProfit float64 `json:"net_profit"`
	} `json:"cumulative"`
}

type ArbitrageDepthAnalysis struct {
	Currency              string            `json:"currency"`
	BuyMarket             EnhancedOrderBook `json:"buy_market"`
	SellMarket            EnhancedOrderBook `json:"sell_market"`
	OrderSimulations      []OrderSimulation `json:"order_simulations"`
	MaxProfitableOrders   int               `json:"max_profitable_orders"`
	TotalProfitableVolume float64           `json:"total_profitable_volume"`
	TotalEstimatedProfit  float64           `json:"total_estimated_profit"`
	BottleneckSide        string            `json:"bottleneck_side"`
	OpportunityRating     string            `json:"opportunity_rating"`
	Timestamp             time.Time         `json:"timestamp"`
}

// Configuration
type Config struct {
	MinNetMargin    float64       `json:"min_net_margin"`
	MinLiquidity    float64       `json:"min_liquidity"`
	FeeRate         float64       `json:"fee_rate"`
	MaxOrderLevels  int           `json:"max_order_levels"`
	CacheDuration   time.Duration `json:"cache_duration"`
	RateCacheFile   string        `json:"rate_cache_file"`
	ValidCurrencies []string      `json:"valid_currencies"`
	EnableAllPairs  bool          `json:"enable_all_pairs"`
}

// Default configuration
func DefaultConfig() *Config {
	return &Config{
		MinNetMargin:    2.0,
		MinLiquidity:    100.0,
		FeeRate:         0.02,
		MaxOrderLevels:  10,
		CacheDuration:   5 * time.Minute,
		RateCacheFile:   "exchange_rates.json",
		ValidCurrencies: []string{"INR", "USDT", "BTC", "ETH", "BNB", "BUSD", "USDC"},
		EnableAllPairs:  false,
	}
}

// Execution Configuration
type ExecutionConfig struct {
	MaxPositionUSDT     float64 `json:"max_position_usdt"`     // Maximum position size in USDT
	MinRequiredUSDT     float64 `json:"min_required_usdt"`     // Minimum USDT balance required
	StopLossPct         float64 `json:"stop_loss_pct"`         // Stop loss threshold percentage
	OrderTimeoutSeconds int     `json:"order_timeout_seconds"` // Order fill timeout
	DelayBetweenOrders  int     `json:"delay_between_orders"`  // Delay between orders in milliseconds
	UseMarketOrders     bool    `json:"use_market_orders"`     // Use market orders vs limit orders
	MaxOrdersPerRun     int     `json:"max_orders_per_run"`    // Maximum orders to execute per run
	RiskToleranceLevel  string  `json:"risk_tolerance_level"`  // conservative, moderate, aggressive
}

// Default execution configuration
func DefaultExecutionConfig() *ExecutionConfig {
	return &ExecutionConfig{
		MaxPositionUSDT:     100.0, // Start with $100 max position
		MinRequiredUSDT:     10.0,  // Require at least $10 USDT
		StopLossPct:         3.0,   // 3% stop loss as requested
		OrderTimeoutSeconds: 30,    // 30 second timeout per order
		DelayBetweenOrders:  2000,  // 2 second delay between orders
		UseMarketOrders:     true,  // Use market orders for immediate execution
		MaxOrdersPerRun:     5,     // Limit to 5 orders per run initially
		RiskToleranceLevel:  "conservative",
	}
}

// Executed Order Result
type ExecutedOrder struct {
	OrderNumber     int       `json:"order_number"`
	Currency        string    `json:"currency"`
	BuyMarket       string    `json:"buy_market"`
	SellMarket      string    `json:"sell_market"`
	BuyOrderID      string    `json:"buy_order_id"`
	SellOrderID     string    `json:"sell_order_id"`
	PlannedVolume   float64   `json:"planned_volume"`
	VolumeExecuted  float64   `json:"volume_executed"`
	BuyPrice        float64   `json:"buy_price"`
	SellPrice       float64   `json:"sell_price"`
	ExpectedProfit  float64   `json:"expected_profit"`
	ActualProfit    float64   `json:"actual_profit"`
	ActualMarginPct float64   `json:"actual_margin_pct"`
	Success         bool      `json:"success"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	ExecutionTimeMs int64     `json:"execution_time_ms"`
}

// Complete Execution Result
type ExecutionResult struct {
	Currency        string          `json:"currency"`
	BuyMarket       string          `json:"buy_market"`
	SellMarket      string          `json:"sell_market"`
	StartTime       time.Time       `json:"start_time"`
	EndTime         time.Time       `json:"end_time"`
	TotalProfit     float64         `json:"total_profit"`
	TotalVolume     float64         `json:"total_volume"`
	TotalInvestment float64         `json:"total_investment"`
	Orders          []ExecutedOrder `json:"orders"`
	Successful      bool            `json:"successful"`
	Timestamp       time.Time       `json:"timestamp"`
	Config          ExecutionConfig `json:"config"`
}
