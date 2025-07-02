package arbitrage

// ArbitrageOpportunity represents a simple 2-step arbitrage trade
type ArbitrageOpportunity struct {
	// Trade details
	SourcePair     string `json:"source_pair"`     // e.g., "B-BTC_USDT"
	TargetPair     string `json:"target_pair"`     // e.g., "B-BTC_INR" or "B-BTC_BTC" or "B-BTC_ETH"
	Coin           string `json:"coin"`            // e.g., "BTC"
	SourceCurrency string `json:"source_currency"` // Always "USDT"
	TargetCurrency string `json:"target_currency"` // "INR", "BTC", or "ETH"

	// Price information
	SourceBuyPrice  float64 `json:"source_buy_price"`  // Price to buy coin with USDT
	TargetSellPrice float64 `json:"target_sell_price"` // Price to sell coin for target currency

	// Volume information
	SourceBuyVolume  float64 `json:"source_buy_volume"`  // Available volume at source buy price
	TargetSellVolume float64 `json:"target_sell_volume"` // Available volume at target sell price
	MaxTradeVolume   float64 `json:"max_trade_volume"`   // Maximum tradeable volume (min of both)

	// Profit calculations
	GrossProfit   float64 `json:"gross_profit"`   // Profit before fees and taxes
	TradingFees   float64 `json:"trading_fees"`   // Total trading fees (both legs)
	TDSAmount     float64 `json:"tds_amount"`     // 1% TDS (if applicable)
	NetProfit     float64 `json:"net_profit"`     // Profit after fees and TDS
	ProfitPercent float64 `json:"profit_percent"` // Profit percentage

	// Tax implications
	TaxableAmount float64 `json:"taxable_amount"` // Amount subject to 30% tax
	TaxLiability  float64 `json:"tax_liability"`  // Estimated 30% + 4% cess tax
	FinalProfit   float64 `json:"final_profit"`   // Profit after all costs and taxes

	// Execution details
	IsExecutable  bool    `json:"is_executable"`  // Whether this opportunity is worth executing
	MinInvestment float64 `json:"min_investment"` // Minimum amount needed to execute
	ROI           float64 `json:"roi"`            // Return on investment percentage
}

// FeeStructure represents the fee structure for different trading volumes
type FeeStructure struct {
	Level           string  `json:"level"`            // e.g., "Regular 1", "VIP Level 1"
	SpotINRFee      float64 `json:"spot_inr_fee"`     // Spot INR trading fee
	SpotC2CFee      float64 `json:"spot_c2c_fee"`     // Crypto-to-crypto trading fee
	VolumeThreshold float64 `json:"volume_threshold"` // 30-day volume threshold for this level
}

// TradingContext represents the current trading context
type TradingContext struct {
	UserVolume30Day float64      `json:"user_volume_30day"` // User's 30-day trading volume
	CurrentFeeLevel FeeStructure `json:"current_fee_level"` // Current applicable fee structure
	USDTBalance     float64      `json:"usdt_balance"`      // Available USDT balance
	HasTDSThreshold bool         `json:"has_tds_threshold"` // Whether user exceeds TDS threshold
}

// MarketPair represents a trading pair with its details
type MarketPair struct {
	Pair                string   `json:"pair"`
	BaseCurrency        string   `json:"base_currency"`
	TargetCurrency      string   `json:"target_currency"`
	Status              string   `json:"status"`
	MinQuantity         float64  `json:"min_quantity"`
	MaxQuantity         float64  `json:"max_quantity"`
	MinNotional         float64  `json:"min_notional"`
	AvailableOrderTypes []string `json:"available_order_types"`
	IsActive            bool     `json:"is_active"`
}

// ArbitrageMatrix represents all possible arbitrage opportunities
type ArbitrageMatrix struct {
	USDTPairs          []MarketPair            `json:"usdt_pairs"`          // All pairs with USDT as base
	TargetPairs        map[string][]MarketPair `json:"target_pairs"`        // Pairs grouped by target currency (INR, BTC, ETH)
	Opportunities      []ArbitrageOpportunity  `json:"opportunities"`       // All found opportunities
	TotalPairs         int                     `json:"total_pairs"`         // Total number of pairs analyzed
	TotalOpportunities int                     `json:"total_opportunities"` // Total opportunities found
	AnalysisTimestamp  int64                   `json:"analysis_timestamp"`  // When this analysis was done
}

// Constants for fee calculations
const (
	// Trading Fees (Regular 1 tier - worst case scenario)
	SpotINRFeeRegular1 = 0.005  // 0.50%
	SpotC2CFeeRegular1 = 0.0017 // 0.17%

	// Tax rates
	TDSRate         = 0.01 // 1% TDS
	CapitalGainsTax = 0.30 // 30% capital gains tax
	CessRate        = 0.04 // 4% cess on capital gains tax
	GST             = 0.18 // 18% GST (already included in fees)

	// TDS thresholds
	TDSThresholdGeneral = 50000 // ₹50,000 for general users
	TDSThresholdSpecial = 10000 // ₹10,000 for special cases

	// Minimum profit thresholds
	MinProfitThreshold = 0.02 // 2% minimum profit to consider viable
	MinTradeAmount     = 100  // ₹100 minimum trade amount
)
