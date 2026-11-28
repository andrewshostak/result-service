package googleauth

import (
	"context"

	"google.golang.org/api/idtoken"
)

type NoopValidator struct{}

func NewNoopValidator() *NoopValidator {
	return &NoopValidator{}
}

func (*NoopValidator) Validate(context.Context, string, string) (*idtoken.Payload, error) {
	return &idtoken.Payload{}, nil
}
