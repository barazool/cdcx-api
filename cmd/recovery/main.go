package main

import (
	"fmt"
	"log"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/coindcx"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmt.Println("🔄 CoinDCX Recovery Tool")
	fmt.Println("========================")
	fmt.Println("💰 Converting VET back to USDT...")

	// Load API configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Error loading config: %v", err)
	}

	// Create client
	client := coindcx.NewClient(cfg.APIKey, cfg.APISecret)

	// Check current balances
	fmt.Println("\n🔍 Checking current balances...")
	balances, err := client.GetBalances()
	if err != nil {
		log.Fatalf("❌ Error getting balances: %v", err)
	}

	var vetBalance, usdtBalance float64
	for _, balance := range balances {
		if balance.Currency == "VET" {
			vetBalance = balance.Balance
		}
		if balance.Currency == "USDT" {
			usdtBalance = balance.Balance
		}
	}

	fmt.Printf("💎 VET Balance: %.6f\n", vetBalance)
	fmt.Printf("💰 USDT Balance: %.6f\n", usdtBalance)

	if vetBalance < 1.0 {
		fmt.Println("❌ No VET tokens to sell")
		return
	}

	// Confirm sell
	fmt.Printf("\n⚠️ Sell %.6f VET for USDT? (1=YES, 0=NO): ", vetBalance)
	var choice string
	fmt.Scanln(&choice)

	if choice != "1" {
		fmt.Println("❌ Recovery cancelled")
		return
	}

	// Create SELL order to convert VET back to USDT
	fmt.Println("\n🔄 Placing SELL order: VET → USDT...")

	sellOrder := coindcx.OrderRequest{
		Side:          "sell",
		OrderType:     "market_order",
		Market:        "VETUSDT",
		TotalQuantity: vetBalance,
	}

	response, err := client.CreateOrder(sellOrder)
	if err != nil {
		log.Fatalf("❌ SELL order failed: %v", err)
	}

	if len(response.Orders) == 0 {
		fmt.Println("❌ No order returned")
		return
	}

	order := response.Orders[0]
	fmt.Printf("✅ SELL order placed: %s\n", order.ID)
	fmt.Printf("📊 Order Status: %s\n", order.Status)
	fmt.Printf("💰 Selling: %.6f VET\n", order.TotalQuantity)

	// Check order status after a moment
	fmt.Println("\n⏳ Checking order status...")

	// Wait a moment for order to process
	fmt.Println("Waiting 3 seconds...")
	// time.Sleep(3 * time.Second) - commenting out for safety

	finalOrder, err := client.GetOrderStatus(order.ID)
	if err != nil {
		log.Printf("⚠️ Could not check final status: %v", err)
		fmt.Printf("🔍 Check order status manually: %s\n", order.ID)
	} else {
		fmt.Printf("📊 Final Status: %s\n", finalOrder.Status)
		fmt.Printf("💰 Average Price: ₹%.6f\n", finalOrder.AvgPrice)
		fmt.Printf("📈 Remaining: %.6f VET\n", finalOrder.RemainingQuantity)
	}

	// Check final balances
	fmt.Println("\n🔍 Checking final balances...")
	finalBalances, err := client.GetBalances()
	if err != nil {
		log.Printf("⚠️ Could not get final balances: %v", err)
		return
	}

	var finalVET, finalUSDT float64
	for _, balance := range finalBalances {
		if balance.Currency == "VET" {
			finalVET = balance.Balance
		}
		if balance.Currency == "USDT" {
			finalUSDT = balance.Balance
		}
	}

	fmt.Printf("💎 Final VET: %.6f\n", finalVET)
	fmt.Printf("💰 Final USDT: %.6f\n", finalUSDT)
	fmt.Printf("📈 USDT Change: %.6f\n", finalUSDT-usdtBalance)

	if finalOrder != nil && finalOrder.Status == "filled" {
		fmt.Println("✅ Recovery successful! VET converted back to USDT")
	} else {
		fmt.Println("⚠️ Recovery in progress. Check your balance in a few minutes.")
	}

	fmt.Println("\n🎯 Recovery complete!")
}
