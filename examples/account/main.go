// Command account fetches balances, fees, and limits.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mintarex/mintarex-go"
)

func main() {
	mx, err := mintarex.New(mintarex.Options{
		APIKey:    os.Getenv("MX_KEY"),
		APISecret: os.Getenv("MX_SECRET"),
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	balances, err := mx.Account.Balances(ctx, mintarex.BalancesParams{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Balances:")
	for _, b := range balances.Balances {
		fmt.Printf("  %-6s available=%s locked=%s\n", b.Currency, b.Available, b.Locked)
	}

	fees, err := mx.Account.Fees(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nFees: trading=%s\n", fees.TradingFeeRate)

	limits, err := mx.Account.Limits(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Account type: %s\n", limits.AccountType)
}
