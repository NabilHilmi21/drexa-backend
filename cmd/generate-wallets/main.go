package main

import (
	"context"
	"fmt"
	"log"

	"drexa/internal/wallet/service"
	"drexa/pkg/config"
)

func main() {
	cfg := config.Load()
	if cfg.Tatum.APIKey == "" {
		log.Fatal("Tatum API key not found in environment (set TATUM_TESTNET_API_KEY or TATUM_WALLET_API_KEY)")
	}

	tatumSvc := service.NewTatumService(cfg.Tatum, "https://api.tatum.io")
	ctx := context.Background()

	chains := []string{"bitcoin", "ethereum", "bsc", "solana"}

	fmt.Println("Generating master HD wallets via Tatum...")
	fmt.Println("-------------------------------------------")

	for _, chain := range chains {
		xpub, err := tatumSvc.GenerateWallet(ctx, chain)
		if err != nil {
			log.Printf("Failed to generate wallet for %s: %v", chain, err)
			continue
		}
		
		envVar := ""
		switch chain {
		case "bitcoin":
			envVar = "BTC_MASTER_XPUB"
		case "ethereum":
			envVar = "ETH_MASTER_XPUB"
		case "bsc":
			envVar = "BNB_MASTER_XPUB"
		case "solana":
			envVar = "SOL_HOT_ADDRESS"
			// Note: for Solana, Tatum often returns `address` not `xpub`. This script might not capture it if Tatum returns a different JSON structure for Solana, so we'll provide manual instructions below as fallback.
		}
		
		if xpub != "" {
			fmt.Printf("%s=%s\n", envVar, xpub)
		}
	}
	
	fmt.Println("\nFor Solana, since it uses a single hot-wallet address, if the above didn't print it:")
	fmt.Println("Please manually generate or provide a Solana address and set:")
	fmt.Println("SOL_HOT_ADDRESS=your_solana_address_here")
	
	fmt.Println("\nAdd the above variables to your drexa-backend/.env file to fix the wallet address generation.")
}
