package app

import "app/internal/env"

type Bootstrap struct {
	Port string
}

func InitBootstrap() Bootstrap {
	return Bootstrap{Port: env.GetEnv().PORT}
}
