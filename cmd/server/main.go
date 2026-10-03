package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Printf("Listening on http://localhost%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
