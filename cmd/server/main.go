package main

import (
	"log"
	"net/http"
	"os"

	"github.com/carlyleec/go-ssh-term/internal/web"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", web.Handler(os.DirFS("frontend/dist")))

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Printf("Listening on http://localhost%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
