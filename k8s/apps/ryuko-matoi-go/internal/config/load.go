package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

func Load() (*Config, error) {
	_ = godotenv.Load()
	_ = godotenv.Load("configs/.env")

	eventBufferSize, err := getEnvInt("WHATSAPP_EVENT_BUFFER_SIZE", 64)
	if err != nil {
		return nil, fmt.Errorf("parse WHATSAPP_EVENT_BUFFER_SIZE: %w", err)
	}

	cfg := &Config{
		AppName:     getEnvOrDefault("APP_NAME", "ryuko-matoi"),
		Environment: getEnvOrDefault("APP_ENV", "development"),
		LogLevel:    getEnvOrDefault("LOG_LEVEL", "info"),
		WhatsApp: &WhatsAppConfig{
			SessionPath:     getEnvOrDefault("WHATSAPP_SESSION_PATH", "./data/session"),
			DeviceName:      getEnvOrDefault("WHATSAPP_DEVICE_NAME", "ryuko-matoi-bot"),
			DatabaseDialect: getEnvOrDefault("WHATSAPP_DATABASE_DIALECT", "sqlite"),
			DatabaseDsn:     getEnvOrDefault("WHATSAPP_DATABASE_DSN", "file:whatsmeow.db?_pragma=foreign_keys(1)"),
			PairingPhone:    os.Getenv("WHATSAPP_PAIRING_PHONE"),
			EventBufferSize: int32(eventBufferSize),
		},
		Ai: &AIConfig{
			Provider: getEnvOrDefault("AI_PROVIDER", "google-genai"),
			ApiKey:   firstNonEmptyEnv("AI_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY"),
			Model:    getEnvOrDefault("AI_MODEL", "gemini-2.5-flash"),
		},
		Ocr: &OCRConfig{
			Provider: getEnvOrDefault("OCR_PROVIDER", "gosseract"),
			Language: getEnvOrDefault("OCR_LANGUAGE", "eng"),
			Binary:   getEnvOrDefault("OCR_BINARY", "tesseract"),
		},
		Api: &APIConfig{
			RemoveBgUrl:    os.Getenv("REMOVE_BG_API_URL"),
			JokesUrl:       os.Getenv("JOKES_API_URL"),
			AnimeQuoteUrl:  os.Getenv("ANIME_QUOTE_API_URL"),
			DistroInfoUrl:  os.Getenv("DISTRO_INFO_API_URL"),
			DoaUrl:         os.Getenv("DOA_API_URL"),
			QuranUrl:       os.Getenv("QURAN_API_URL"),
			AsmaulHusnaUrl: os.Getenv("ASMAUL_HUSNA_API_URL"),
		},
		Moderation: &ModerationConfig{
			BlacklistWords: parseCSVEnv("BLACKLIST_WORDS"),
		},
		Media: &MediaConfig{
			RemoveBgApiKey: os.Getenv("REMOVE_BG_API_KEY"),
		},
	}

	return cfg, nil
}

func getEnvOrDefault(key string, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		return defaultValue
	}

	return value
}

func getEnvInt(key string, defaultValue int) (int, error) {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

func parseCSVEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	rawParts := strings.Split(value, ",")
	words := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		normalized := strings.ToLower(strings.TrimSpace(strings.Trim(part, `"'`)))
		if normalized == "" {
			continue
		}
		words = append(words, normalized)
	}

	return words
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}

	return ""
}
