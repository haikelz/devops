package app

import (
	"app/internal/app"
	"app/internal/config"
	"net/http"
	"sync"

	_ "app/docs"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
)

var (
	handlerOnce sync.Once
	handler     *echo.Echo
)

// Handle deployment to Vercel Serverless
// @title Ekel Backend
// @version 1.0
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter the token with the `BearerAuth` prefix."
func Handler(w http.ResponseWriter, r *http.Request) {
	handlerOnce.Do(func() {
		config.InitLogger()
		db := config.InitGorm()

		e := echo.New()
		app.ConfigureServer(e)
		routeGroups := app.BuildRouteGroups(e)
		app.RegisterModules(e, db, routeGroups)

		handler = e
	})

	handler.ServeHTTP(w, r)
}
