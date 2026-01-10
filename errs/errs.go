package errs

type code string

const (
	CodeResourceNotFound      code = "resource_not_found"
	CodeResourceAlreadyExists code = "resource_already_exists"
	CodeUnprocessableContent  code = "unprocessable_content"
	CodeInternalServerError   code = "internal_server_error"
	CodeInvalidRequest        code = "invalid_request"
	CodeTimeout               code = "timeout"
)

func NewResourceNotFoundError(error error) ResourceNotFoundError {
	return ResourceNotFoundError{Err: error, Code: CodeResourceNotFound}
}

type ResourceNotFoundError struct {
	Err  error
	Code code
}

func (e ResourceNotFoundError) Error() string {
	return e.Err.Error()
}

func NewResourceAlreadyExistsError(error error) ResourceAlreadyExistsError {
	return ResourceAlreadyExistsError{Err: error, Code: CodeResourceAlreadyExists}
}

type ResourceAlreadyExistsError struct {
	Err  error
	Code code
}

func (e ResourceAlreadyExistsError) Error() string {
	return e.Err.Error()
}

func NewUnprocessableContentError(error error) UnprocessableContentError {
	return UnprocessableContentError{Err: error, Code: CodeUnprocessableContent}
}

type UnprocessableContentError struct {
	Err  error
	Code code
}

func (e UnprocessableContentError) Error() string {
	return e.Err.Error()
}
