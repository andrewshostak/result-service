package fotmob

import (
	"net/http"

	"github.com/rs/zerolog"
)

type Logger interface {
	Error() *zerolog.Event
}

type HTTPManager interface {
	Do(req *http.Request) (*http.Response, error)
}
