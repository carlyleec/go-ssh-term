package connections

import (
	"context"

	"github.com/carlyleec/go-ssh-term/internal/auth"
)

type HostOutput struct{ Body HostInspection }
type HostDecisionInput struct {
	ID   string `path:"id"`
	Body HostDecision
}

func (d *Dialer) InspectHost(ctx context.Context, input *DeleteInput) (*HostOutput, error) {
	account, _ := auth.AccountFromContext(ctx)
	result, err := d.Inspect(ctx, account.ID, input.ID)
	if err != nil {
		return nil, err
	}
	return &HostOutput{Body: result}, nil
}

func (d *Dialer) ApproveHost(ctx context.Context, input *HostDecisionInput) (*HostOutput, error) {
	account, _ := auth.AccountFromContext(ctx)
	result, err := d.Approve(ctx, account.ID, input.ID, input.Body)
	if err != nil {
		return nil, err
	}
	return &HostOutput{Body: result}, nil
}

func (d *Dialer) ResetHostTrust(ctx context.Context, input *HostDecisionInput) (*struct{}, error) {
	account, _ := auth.AccountFromContext(ctx)
	if err := d.ResetTrust(ctx, account.ID, input.ID, input.Body); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}
