package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var Log zerolog.Logger

func Init(isDev bool) {
	zerolog.TimeFieldFormat = time.RFC3339

	if isDev {
		Log = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
		}).With().Timestamp().Logger()
	} else {
		Log = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}
}

func IsDev() bool {
	env := os.Getenv("ENV")
	return env == "" || env == "dev" || env == "development"
}
