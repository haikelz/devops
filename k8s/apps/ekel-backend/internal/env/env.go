package env

import "os"

type DtoEnv struct {
	PORT                string
	APP_ENV             string
	APP_DEBUG           string
	TURSO_DATABASE_URL  string
	TURSO_AUTH_TOKEN    string
	SECRET_KEY_ADMIN    string
	SECRET_KEY_CUSTOMER string
}

func GetEnv() DtoEnv {
	return DtoEnv{
		PORT:                getEnv("PORT", "9090"),
		APP_ENV:             getEnv("APP_ENV", "development"),
		APP_DEBUG:           getEnv("APP_DEBUG", "false"),
		TURSO_DATABASE_URL:  getEnv("TURSO_DATABASE_URL", ""),
		TURSO_AUTH_TOKEN:    getEnv("TURSO_AUTH_TOKEN", ""),
		SECRET_KEY_ADMIN:    getEnv("SECRET_KEY_ADMIN", "change-me-admin-secret"),
		SECRET_KEY_CUSTOMER: getEnv("SECRET_KEY_CUSTOMER", "change-me-customer-secret"),
	}
}

func getEnv(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}
