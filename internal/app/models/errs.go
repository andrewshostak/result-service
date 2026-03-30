package models

type Code string

const (
	CodeResourceNotFound      Code = "resource_not_found"
	CodeResourceAlreadyExists Code = "resource_already_exists"
	CodeUnprocessableContent  Code = "unprocessable_content"
	CodeInternalServerError   Code = "internal_server_error"
	CodeInvalidRequest        Code = "invalid_request"
	CodeTimeout               Code = "timeout"
)

func NewResourceNotFoundError(error error) ResourceNotFoundError {
	return ResourceNotFoundError{Err: error, Code: CodeResourceNotFound}
}

type ResourceNotFoundError struct {
	Err  error
	Code Code
}

func (e ResourceNotFoundError) Error() string {
	return e.Err.Error()
}

func NewResourceAlreadyExistsError(error error) ResourceAlreadyExistsError {
	return ResourceAlreadyExistsError{Err: error, Code: CodeResourceAlreadyExists}
}

type ResourceAlreadyExistsError struct {
	Err  error
	Code Code
}

func (e ResourceAlreadyExistsError) Error() string {
	return e.Err.Error()
}

func NewUnprocessableContentError(error error) UnprocessableContentError {
	return UnprocessableContentError{Err: error, Code: CodeUnprocessableContent}
}

type UnprocessableContentError struct {
	Err  error
	Code Code
}

func (e UnprocessableContentError) Error() string {
	return e.Err.Error()
}
