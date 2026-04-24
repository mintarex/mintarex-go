// Command rfq requests a BTC/USD quote and executes it.
package main

import (
	"context"
	"errors"
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

	quote, err := mx.RFQ.Quote(ctx, mintarex.QuoteRequest{
		Base: "BTC", Quote: "USD", Side: "buy",
		Amount: "0.001", AmountType: "base",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("quote_id=%s price=%s expires_in_ms=%d\n",
		quote.QuoteID, quote.Price, quote.ExpiresInMs)

	trade, err := mx.RFQ.Accept(ctx, quote.QuoteID, mintarex.AcceptOptions{})
	var qe *mintarex.QuoteExpiredError
	if errors.As(err, &qe) {
		fmt.Println("quote expired before accept — re-quote and retry")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("filled: %s %s\n", trade.TradeID, trade.Status)
}
