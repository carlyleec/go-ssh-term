package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/connections"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
)

func main() {
	if err := export(os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func export(w io.Writer) error {
	contract := api.New(http.NewServeMux())
	var access auth.Access
	sshkeys.NewHandler(nil, nil).Register(contract, &access)
	connections.NewHandler(nil).Register(contract, &access)
	access.RegisterCurrentUser(contract)
	access.RegisterLogout(contract)
	auth.NewRegistration(nil, nil, nil).Register(contract, "")
	auth.NewLogin(nil, nil, nil).Register(contract, "")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(contract.OpenAPI())
}
