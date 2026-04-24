// Command streams listens to the price SSE stream.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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

	stream, err := mx.Streams.Prices(ctx, mintarex.StreamOptions{AutoReconnect: true})
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	fmt.Println("Listening for price updates (Ctrl-C to stop)...")
	for {
		msg, err := stream.Next(ctx)
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %v\n", msg.Event, msg.Data)
	}
}
