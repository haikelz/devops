// @title Ekel Backend API
// @version 1.0
// @description API for Wakatime stats, IHSG market data, and Guestbook.
// @host localhost:9090
// @BasePath /api/v1

package main

import (
	_ "app/docs"
	"app/internal/app"
	"app/internal/config"

	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
)

func main() {
	config.InitLogger()
	db := config.InitGorm()
	bootstrap := app.InitBootstrap()

	e := echo.New()
	app.ConfigureServer(e)
	routeGroups := app.BuildRouteGroups(e)
	app.RegisterModules(e, db, routeGroups)
	app.StartAndWaitShutdown(e, bootstrap.Port)
}
