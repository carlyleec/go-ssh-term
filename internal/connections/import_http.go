package connections

import "context"

type ImportRequest struct {
	Config string `json:"config" maxLength:"65536"`
}
type ImportInput struct{ Body ImportRequest }
type ImportOutput struct{ Body ImportPreview }

func (h *handler) previewImport(_ context.Context, input *ImportInput) (*ImportOutput, error) {
	if len(input.Body.Config) > MaxImportBytes {
		return nil, failure(413, "config exceeds 64 KiB")
	}
	return &ImportOutput{Body: parseImport(input.Body.Config)}, nil
}
