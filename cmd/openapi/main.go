package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
)

func main() {
	if err := export(os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func export(w io.Writer) error {
	contract := api.New(http.NewServeMux())
	var access auth.Access
	access.RegisterCurrentUser(contract)
	access.RegisterLogout(contract)
	auth.NewRegistration(nil, nil, nil).Register(contract, "")
	auth.NewLogin(nil, nil, nil).Register(contract, "")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(contract.OpenAPI())
}
