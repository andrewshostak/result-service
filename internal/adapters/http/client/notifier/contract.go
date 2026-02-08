package notifier

import "github.com/rs/zerolog"

type Logger interface {
	Error() *zerolog.Event
}
