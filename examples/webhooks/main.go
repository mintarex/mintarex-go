// Command webhooks runs an HTTP server that verifies inbound Mintarex
// webhook deliveries and prints the parsed events.
package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/mintarex/mintarex-go"
)

func main() {
	secret := os.Getenv("MINTAREX_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("set MINTAREX_WEBHOOK_SECRET")
	}

	http.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		event, err := mintarex.VerifyWebhook(mintarex.VerifyParams{
			Body: body, Headers: r.Header, Secret: secret,
		})
		if err != nil {
			log.Println("webhook verify failed:", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		log.Printf("%s: %s %v\n", event.EventType, event.EventID, event.Data)
		w.WriteHeader(http.StatusNoContent)
	})

	log.Println("listening on :8000/hook ...")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal(err)
	}
}
