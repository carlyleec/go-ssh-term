package auth

import (
	"context"
)

type CurrentUserOutput struct {
	Body struct {
		Account Account `json:"account"`
	}
}

// GetCurrent returns the account supplied by verified-account middleware.
func (a *Access) GetCurrent(ctx context.Context, _ *struct{}) (*CurrentUserOutput, error) {
	output := &CurrentUserOutput{}
	output.Body.Account, _ = AccountFromContext(ctx)
	return output, nil
}
