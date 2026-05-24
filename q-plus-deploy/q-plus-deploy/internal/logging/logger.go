package logging

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/samber/do/v2"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
	"io"
	"os"
)

type LogConfig struct {
	Debug    bool
	GelfAddr string
}

func NewLogger(i do.Injector) (*zerolog.Logger, error) {
	config := do.MustInvoke[LogConfig](i)

	var writer io.Writer = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"}
	if config.GelfAddr != "" {
		gelfLogger, err := gelf.NewTCPWriter(config.GelfAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to create gelf logger: %w", err)
		}
		writer = io.MultiWriter(writer, gelfLogger)
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	logger := zerolog.New(writer).With().Timestamp().Logger()

	if config.Debug {
		logger = logger.Level(zerolog.TraceLevel).With().Caller().Logger()
	} else {
		logger = logger.Level(zerolog.InfoLevel)
	}

	return &logger, nil
}
