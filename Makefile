# CoinDCX Arbitrage System
.PHONY: help pairs opportunities depth all clean test

help: ## Show this help message
	@echo "🚀 CoinDCX Arbitrage System"
	@echo "=========================="
	@echo ""
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

pairs: ## Step 1: Detect arbitrage pairs
	@echo "🔍 Step 1: Detecting arbitrage pairs..."
	go run cmd/pair-detector/main.go

opportunities: ## Step 2: Find arbitrage opportunities (requires pairs)
	@echo "🚀 Step 2: Finding arbitrage opportunities..."
	go run cmd/opportunity-detector/main.go

depth: ## Step 3: Analyze order book depth (requires opportunities)
	@echo "🔬 Step 3: Analyzing order book depth..."
	go run cmd/depth-analyzer/main.go

all: pairs opportunities depth ## Run complete arbitrage analysis pipeline

all-pairs: ## Run pipeline with all currency pairs enabled
	@echo "🌐 Running complete analysis with ALL pairs enabled..."
	ENABLE_ALL_PAIRS=true go run cmd/pair-detector/main.go
	go run cmd/opportunity-detector/main.go
	go run cmd/depth-analyzer/main.go

test: ## Test API connection
	go run cmd/test/main.go

convert: ## Convert INR to USDT (manual trading)
	go run cmd/converter/main.go

clean: ## Clean generated files
	@echo "🧹 Cleaning generated files..."
	rm -f arbitrage_pairs.json
	rm -f arbitrage_opportunities.json
	rm -f depth_analysis.json
	rm -f exchange_rates.json

deps: ## Install dependencies
	go mod tidy
	go mod download

build: ## Build all binaries
	@echo "🔨 Building binaries..."
	go build -o bin/pair-detector cmd/pair-detector/main.go
	go build -o bin/opportunity-detector cmd/opportunity-detector/main.go
	go build -o bin/depth-analyzer cmd/depth-analyzer/main.go
	go build -o bin/converter cmd/converter/main.go
	go build -o bin/test cmd/test/main.go

# Configuration examples
config-help: ## Show configuration options
	@echo "⚙️  Configuration Options:"
	@echo "========================"
	@echo ""
	@echo "Environment Variables:"
	@echo "  ENABLE_ALL_PAIRS=true     # Include all currency pairs (not just major ones)"
	@echo "  MIN_NET_MARGIN=1.5        # Minimum net margin percentage (default: 2.0)"
	@echo "  MIN_LIQUIDITY=50          # Minimum liquidity in INR (default: 100.0)"
	@echo ""
	@echo "Examples:"
	@echo "  ENABLE_ALL_PAIRS=true make pairs"
	@echo "  MIN_NET_MARGIN=1.5 make opportunities"
	@echo "  MIN_LIQUIDITY=50 MIN_NET_MARGIN=1.0 make all"

# Development helpers
fmt: ## Format Go code
	go fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

watch: ## Watch for changes and run analysis
	@echo "👀 Watching for changes... (Press Ctrl+C to stop)"
	while true; do \
		make all; \
		echo "⏳ Waiting 30 seconds..."; \
		sleep 30; \
	done
