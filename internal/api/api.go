// Package api configures the HTTP contract shared by runtime and export.
package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

func New(mux *http.ServeMux) huma.API {
	cfg := huma.DefaultConfig("Browser SSH Gateway", "1.0.0")
	// Preserve response envelopes without adding $schema fields or schema links.
	cfg.CreateHooks = nil
	// Serve only explicitly registered application routes during the migration.
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"session": {Type: "apiKey", In: "cookie", Name: "ssh_term_session"},
	}
	return humago.New(mux, cfg)
}
