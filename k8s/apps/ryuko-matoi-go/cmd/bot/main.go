package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/rs/zerolog"

	"ryuko-matoi/internal/config"
	aiInfra "ryuko-matoi/internal/infra/ai"
	facebookInfra "ryuko-matoi/internal/infra/facebook"
	"ryuko-matoi/internal/infra/httpclient"
	instagramInfra "ryuko-matoi/internal/infra/instagram"
	appLogger "ryuko-matoi/internal/infra/logger"
	ocrInfra "ryuko-matoi/internal/infra/ocr"
	twitterInfra "ryuko-matoi/internal/infra/twitter"
	waInfra "ryuko-matoi/internal/infra/whatsapp"
	"ryuko-matoi/internal/repositories"
	"ryuko-matoi/internal/services"
)

const appTimezone = "Asia/Jakarta"
const jakartaOffsetSeconds = 7 * 60 * 60

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	location, err := time.LoadLocation(appTimezone)
	if err != nil {
		location = time.FixedZone(appTimezone, jakartaOffsetSeconds)
	}

	time.Local = location
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().In(location)
	}

	logger, loggerCloser, err := appLogger.NewJournalLogger(appLogger.Options{
		AppName:       cfg.AppName,
		Environment:   cfg.Environment,
		Level:         cfg.LogLevel,
		LogDir:        os.Getenv("LOG_DIR"),
		EnableConsole: true,
		Location:      location,
	})
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer func() {
		if closeErr := loggerCloser.Close(); closeErr != nil {
			logger.Error().Err(closeErr).Msg("close journal logger")
		}
	}()

	log.SetFlags(0)
	log.SetOutput(logger)

	whatsAppRuntime, err := waInfra.NewRuntime(ctx, waInfra.RuntimeDependencies{
		Config: &waInfra.Config{
			DatabaseDialect: cfg.WhatsApp.DatabaseDialect,
			DatabaseDsn:     cfg.WhatsApp.DatabaseDsn,
			PairingPhone:    cfg.WhatsApp.PairingPhone,
			ClientName:      cfg.WhatsApp.DeviceName,
			EventBufferSize: cfg.WhatsApp.EventBufferSize,
		},
		Logger: waInfra.NewZerologLogger(logger, "whatsmeow"),
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("setup whatsapp runtime")
	}

	var aiClient aiInfra.Client
	if cfg.Ai != nil && strings.TrimSpace(cfg.Ai.ApiKey) != "" {
		aiClient, err = aiInfra.NewClientFromConfig(cfg.Ai)
		if err != nil {
			logger.Fatal().Err(err).Msg("setup ai client")
		}
	}

	var ocrClient ocrInfra.Client
	if cfg.Ocr.Provider == "gosseract" || cfg.Ocr.Provider == "tesseract" || cfg.Ocr.Provider == "" {
		ocrClient = ocrInfra.NewTesseractClient(cfg.Ocr.Binary, cfg.Ocr.Language)
	}

	httpClient := httpclient.NewRestClient(20 * time.Second)

	financeDbPath := os.Getenv("FINANCE_DATABASE_PATH")
	if financeDbPath == "" {
		financeDbPath = "finance.db"
	}
	financeRepo, err := repositories.NewFinanceRepository(financeDbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("setup finance repository")
	}

	scheduleDbPath := os.Getenv("SCHEDULE_DATABASE_PATH")
	if scheduleDbPath == "" {
		scheduleDbPath = "schedule.db"
	}
	scheduleRepo, err := repositories.NewScheduleRepository(scheduleDbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("setup schedule repository")
	}

	whatsAppService := services.NewWhatsAppService(services.WhatsAppServiceDependencies{
		WhatsAppClient:  whatsAppRuntime,
		HTTPClient:      httpClient,
		InstagramClient: instagramInfra.NewDownloader(""),
		TwitterClient:   twitterInfra.NewDownloader(""),
		FacebookClient:  facebookInfra.NewDownloader(""),
		AIClient:        aiClient,
		OCRClient:       ocrClient,
		Config:          cfg,
		FinanceRepo:     financeRepo,
		ScheduleRepo:    scheduleRepo,
	})
	whatsAppService.EnableDefaultMessageHandler()

	var qrChannel <-chan *waInfra.QREvent

	if !whatsAppService.IsAuthenticated() {
		qrChannel, err = whatsAppService.QRChannel(ctx)
		if err != nil {
			logger.Fatal().Err(err).Msg("create qr channel")
		}
	}

	if err := whatsAppService.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("start whatsapp service")
	}

	whatsAppService.StartMonthlyFinanceReportCron(ctx)
	whatsAppService.StartScheduleReminderCron(ctx)

	if qrChannel != nil {
		for event := range qrChannel {
			switch event.Event {
			case "code":
				qrterminal.GenerateHalfBlock(event.Code, qrterminal.L, os.Stdout)
				logger.Info().Msg("Scan QR di atas untuk pairing device ke Whatsapp")
			case "success":
				logger.Info().Msg("Pairing device sukses")
			default:
				if event.Error != "" {
					logger.Error().Str("event", event.Event).Str("error", event.Error).Msg("QR event error")
					continue
				}

				logger.Info().Str("event", event.Event).Msg("QR event")
			}
		}
	} else {
		logger.Info().Msg("Sesi WhatsApp ditemukan, login tanpa QR")
	}

	logger.Info().Msg("WhatsMeow aktif. Tekan Ctrl+C untuk berhenti")

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)
	<-stopSignal

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := whatsAppService.Stop(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("stop whatsapp service")
	}
}
