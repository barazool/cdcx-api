package main

import (
	"fmt"
	"os"

	"github.com/b-thark/cdcx-api/internal/config"
	"github.com/b-thark/cdcx-api/pkg/coindcx"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	client := coindcx.NewClient(cfg.APIKey, cfg.APISecret)

	fmt.Println("CoinDCX API Client - Testing Account Details")
	fmt.Println("==========================================")

	// Test user info
	fmt.Println("\n1. Fetching User Info...")
	userInfo, err := client.GetUserInfo()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ User ID: %s\n", userInfo.CoinDCXID)
		fmt.Printf("   Name: %s %s\n", userInfo.FirstName, userInfo.LastName)
		fmt.Printf("   Email: %s\n", userInfo.Email)
	}

	// Test balances
	fmt.Println("\n2. Fetching Account Balances...")
	balances, err := client.GetBalances()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d currency balances:\n", len(balances))
		for _, balance := range balances {
			if balance.Balance > 0 || balance.Locked > 0 {
				fmt.Printf("   %s: %.8f (Locked: %.8f)\n",
					balance.Currency, balance.Balance, balance.Locked)
			}
		}
	}
}
