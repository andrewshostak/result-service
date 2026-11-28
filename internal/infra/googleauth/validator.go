package googleauth

import (
	"context"

	"google.golang.org/api/idtoken"
)

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (*Validator) Validate(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
	return idtoken.Validate(ctx, idToken, audience)
}
