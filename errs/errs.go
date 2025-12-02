package errs

import "errors"

var (
	ErrIncorrectFixtureStatus          = errors.New("incorrect fixture status")
	ErrUnexpectedAPIFootballStatusCode = errors.New("unexpected status code received from api-football")
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

type ResultTaskAlreadyExistsError struct {
	Message string
}

func (e ResultTaskAlreadyExistsError) Error() string {
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
