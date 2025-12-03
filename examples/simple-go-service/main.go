package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Read secret injected by Rollwave
		secretBytes, err := os.ReadFile("/run/secrets/API_KEY")
		secretValue := "NOT_FOUND"
		if err == nil {
			secretValue = string(secretBytes)
		}

		fmt.Fprintf(w, "🚀 Rollwave Example Service\n")
		fmt.Fprintf(w, "--------------------------\n")
		fmt.Fprintf(w, "Hostname: %s\n", os.Getenv("HOSTNAME"))
		fmt.Fprintf(w, "Secret (API_KEY): %s\n", secretValue)
	})

	fmt.Println("Server listening on :80")
	if err := http.ListenAndServe(":80", nil); err != nil {
		panic(err)
	}
}
