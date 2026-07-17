package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"app/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	echoSwagger "github.com/swaggo/echo-swagger"
)

func ConfigureServer(e *echo.Echo) {
	e.HideBanner = true
	e.HidePort = true
	e.Validator = models.NewCustomValidator()
	e.Server.ReadTimeout = 15 * time.Second
	e.Server.WriteTimeout = 15 * time.Second

	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(middleware.Gzip())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())
	e.Use(middleware.BodyLimit("15M"))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogRemoteIP:  true,
		LogLatency:   true,
		LogRequestID: true,
		LogError:     true,
		HandleError:  true,
		LogValuesFunc: func(_ echo.Context, values middleware.RequestLoggerValues) error {
			entry := logrus.WithFields(logrus.Fields{
				"method":     values.Method,
				"uri":        values.URI,
				"ip":         values.RemoteIP,
				"status":     values.Status,
				"latency":    values.Latency.String(),
				"request_id": values.RequestID,
			})

			if values.Error != nil {
				entry.WithError(values.Error).Error("request failed")
				return nil
			}

			entry.Info("request completed")
			return nil
		},
	}))

	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
	})

	e.GET("/swagger/*", echoSwagger.WrapHandler)
}

func StartAndWaitShutdown(e *echo.Echo, port string) {
	serverErrors := make(chan error, 1)

	go func() {
		logrus.WithField("port", port).Info("starting HTTP server")
		serverErrors <- e.Start(":" + port)
	}()

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-shutdownSignals:
		logrus.WithField("signal", sig.String()).Info("shutdown signal received")
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.WithError(err).Fatal("server failed")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		logrus.WithError(err).Fatal("server shutdown failed")
	}

	logrus.Info("server stopped gracefully")
}
