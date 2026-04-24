// Package mintarex is the official Go SDK for the Mintarex Corporate OTC API.
//
// # Quick start
//
//	import "github.com/mintarex/mintarex-go"
//
//	mx, err := mintarex.New(mintarex.Options{
//	    APIKey:    os.Getenv("MX_KEY"),
//	    APISecret: os.Getenv("MX_SECRET"),
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	quote, err := mx.RFQ.Quote(ctx, mintarex.QuoteRequest{
//	    Base:       "BTC",
//	    Quote:      "USD",
//	    Side:       "buy",
//	    Amount:     "0.5",
//	    AmountType: "base",
//	})
//
// See https://developers.mintarex.com for full API reference.
package mintarex
