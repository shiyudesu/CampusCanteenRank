package errors

const (
	CodeOK           = 0
	CodeBadRequest   = 40001
	CodeUnauthorized = 40101
	CodeForbidden    = 40301
	CodeNotFound     = 40401
	CodeConflict     = 40901
	CodeInternal     = 50000
)

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func New(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}
