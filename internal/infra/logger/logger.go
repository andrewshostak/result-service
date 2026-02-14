package logger

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

func SetupLogger(writers ...io.Writer) *zerolog.Logger {
	writers = append(writers, os.Stderr)
	zerolog.TimeFieldFormat = time.RFC3339Nano
	logger := zerolog.New(zerolog.MultiLevelWriter(writers...)).With().Timestamp().Logger()
	return &logger
}

func GetLogFile() (*os.File, error) {
	filename := "app.log"
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s file to write logs: %w", filename, err)
	}

	return file, nil
}
