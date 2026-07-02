package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type Options struct {
	AppName       string
	Environment   string
	Level         string
	LogDir        string
	EnableConsole bool
	Location      *time.Location
}

type DailyJournalWriter struct {
	mu          sync.Mutex
	logDir      string
	prefix      string
	location    *time.Location
	currentDate string
	file        *os.File
}

func NewJournalLogger(options Options) (zerolog.Logger, io.Closer, error) {
	logDir := strings.TrimSpace(options.LogDir)
	if logDir == "" {
		logDir = "logs"
	}

	location := options.Location
	if location == nil {
		location = time.Local
	}

	journalWriter, err := NewDailyJournalWriter(logDir, "journal-", location)
	if err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("create daily journal writer: %w", err)
	}

	writers := []io.Writer{journalWriter}
	if options.EnableConsole {
		writers = append(writers, zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	}

	level, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(options.Level)))
	if err != nil {
		level = zerolog.InfoLevel
	}

	logger := zerolog.New(io.MultiWriter(writers...)).
		Level(level).
		With().
		Timestamp().
		Str("app", fallback(options.AppName, "ryuko-matoi")).
		Str("env", fallback(options.Environment, "development")).
		Logger()

	return logger, journalWriter, nil
}

func NewDailyJournalWriter(logDir string, prefix string, location *time.Location) (*DailyJournalWriter, error) {
	if strings.TrimSpace(logDir) == "" {
		return nil, fmt.Errorf("log directory is empty")
	}
	if strings.TrimSpace(prefix) == "" {
		return nil, fmt.Errorf("prefix is empty")
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	if location == nil {
		location = time.Local
	}

	return &DailyJournalWriter{
		logDir:   logDir,
		prefix:   prefix,
		location: location,
	}, nil
}

func (writer *DailyJournalWriter) Write(payload []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if err := writer.rotateIfNeeded(time.Now().In(writer.location)); err != nil {
		return 0, err
	}

	if writer.file == nil {
		return 0, fmt.Errorf("journal file is not initialized")
	}

	return writer.file.Write(payload)
}

func (writer *DailyJournalWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if writer.file == nil {
		return nil
	}

	err := writer.file.Close()
	writer.file = nil
	return err
}

func (writer *DailyJournalWriter) rotateIfNeeded(currentTime time.Time) error {
	currentDate := currentTime.Format("02-01-2006")
	if writer.file != nil && writer.currentDate == currentDate {
		return nil
	}

	if writer.file != nil {
		if err := writer.file.Close(); err != nil {
			return fmt.Errorf("close previous journal file: %w", err)
		}
	}

	journalPath := filepath.Join(writer.logDir, fmt.Sprintf("%s%s.log", writer.prefix, currentDate))
	file, err := os.OpenFile(journalPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open journal file: %w", err)
	}

	writer.file = file
	writer.currentDate = currentDate
	return nil
}

func fallback(value string, defaultValue string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return defaultValue
	}
	return normalized
}
