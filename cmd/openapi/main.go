package main

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/connections"
	"github.com/carlyleec/go-ssh-term/internal/routing"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
)

func main() {
	if err := export(os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func export(w io.Writer) error {
	contract := api.New(api.NewRouter())
	routing.Register(contract, routing.Handlers{
		Access:       &auth.Access{},
		Registration: auth.NewRegistration(nil, nil, nil),
		Login:        auth.NewLogin(nil, nil, nil),
		Keys:         sshkeys.NewHandler(nil, nil),
		Connections:  connections.NewHandler(nil),
		Dialer:       connections.NewDialer(nil, nil),
	}, "")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(contract.OpenAPI())
}
