package whatsapp

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type zeroLogger struct {
	logger zerolog.Logger
	module string
}

func NewZerologLogger(logger zerolog.Logger, module string) waLog.Logger {
	return &zeroLogger{
		logger: logger,
		module: strings.TrimSpace(module),
	}
}

func (logger *zeroLogger) Warnf(message string, args ...interface{}) {
	logger.logger.Warn().Str("module", logger.module).Msg(fmt.Sprintf(message, args...))
}

func (logger *zeroLogger) Errorf(message string, args ...interface{}) {
	logger.logger.Error().Str("module", logger.module).Msg(fmt.Sprintf(message, args...))
}

func (logger *zeroLogger) Infof(message string, args ...interface{}) {
	logger.logger.Info().Str("module", logger.module).Msg(fmt.Sprintf(message, args...))
}

func (logger *zeroLogger) Debugf(message string, args ...interface{}) {
	logger.logger.Debug().Str("module", logger.module).Msg(fmt.Sprintf(message, args...))
}

func (logger *zeroLogger) Sub(module string) waLog.Logger {
	baseModule := logger.module
	if baseModule == "" {
		baseModule = strings.TrimSpace(module)
	} else if strings.TrimSpace(module) != "" {
		baseModule = baseModule + "/" + strings.TrimSpace(module)
	}

	return &zeroLogger{
		logger: logger.logger,
		module: baseModule,
	}
}
