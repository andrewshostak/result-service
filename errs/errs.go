package errs

import "errors"

var (
	ErrUnexpectedAPIFootballStatusCode = errors.New("unexpected status code received from api-football")
	ErrUnexpectedFotmobStatusCode      = errors.New("unexpected status code received from fotmob")
	ErrUnexpectedNotifierStatusCode    = errors.New("unexpected status code received from notifier")
)

type AliasNotFoundError struct {
	Message string
}

func (e AliasNotFoundError) Error() string {
	return e.Message
}

type MatchNotFoundError struct {
	Message string
}

func (e MatchNotFoundError) Error() string {
	return e.Message
}

type SubscriptionNotFoundError struct {
	Message string
}

func (e SubscriptionNotFoundError) Error() string {
	return e.Message
}

type CheckResultNotFoundError struct {
	Message string
}

func (e CheckResultNotFoundError) Error() string {
	return e.Message
}

type UnexpectedNumberOfItemsError struct {
	Message string
}

func (e UnexpectedNumberOfItemsError) Error() string {
	return e.Message
}

type SubscriptionAlreadyExistsError struct {
	Message string
}

func (e SubscriptionAlreadyExistsError) Error() string {
	return e.Message
}

type CheckResultTaskAlreadyExistsError struct {
	Message string
}

func (e CheckResultTaskAlreadyExistsError) Error() string {
	return e.Message
}

type ClientTaskAlreadyExistsError struct {
	Message string
}

func (e ClientTaskAlreadyExistsError) Error() string {
	return e.Message
}

type WrongMatchIDError struct {
	Message string
}

func (e WrongMatchIDError) Error() string {
	return e.Message
}

type SubscriptionWrongStatusError struct {
	Message string
}

func (e SubscriptionWrongStatusError) Error() string {
	return e.Message
}
