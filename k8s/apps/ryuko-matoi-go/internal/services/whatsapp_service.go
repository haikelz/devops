package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ryuko-matoi/internal/assets"
	"ryuko-matoi/internal/config"
	"ryuko-matoi/internal/helper"
	"ryuko-matoi/internal/infra/ai"
	"ryuko-matoi/internal/infra/facebook"
	"ryuko-matoi/internal/infra/httpclient"
	"ryuko-matoi/internal/infra/instagram"
	"ryuko-matoi/internal/infra/media_downloader"
	"ryuko-matoi/internal/infra/ocr"
	"ryuko-matoi/internal/infra/tiktok"
	"ryuko-matoi/internal/infra/twitter"
	"ryuko-matoi/internal/infra/whatsapp"
	"ryuko-matoi/internal/repositories"

	"github.com/chai2010/webp"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type commandHandler func(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error)

const waitMessage = "Sedang memproses...."

type WhatsAppService struct {
	dependencies WhatsAppServiceDependencies
	commands     map[string]commandHandler
	mu           sync.RWMutex
	mediaCache   map[string]cachedMedia
}

type cachedMedia struct {
	MediaType string
	MediaMime string
	MediaData []byte
	CreatedAt time.Time
}

type WhatsAppServiceDependencies struct {
	WhatsAppClient  whatsapp.Client
	HTTPClient      httpclient.Client
	InstagramClient instagram.Client
	TikTokClient    tiktok.Client
	TwitterClient   twitter.Client
	FacebookClient  facebook.Client
	AIClient        ai.Client
	OCRClient       ocr.Client
	Config          *config.Config
	FinanceRepo     *repositories.FinanceRepository
	ScheduleRepo    *repositories.ScheduleRepository
}

func NewWhatsAppService(dependencies WhatsAppServiceDependencies) *WhatsAppService {
	service := &WhatsAppService{
		dependencies: dependencies,
		commands:     make(map[string]commandHandler),
		mediaCache:   make(map[string]cachedMedia),
	}

	service.registerCommands()

	return service
}

func (service *WhatsAppService) IsAuthenticated() bool {
	return service.dependencies.WhatsAppClient.IsAuthenticated()
}

func (service *WhatsAppService) QRChannel(ctx context.Context) (<-chan *whatsapp.QREvent, error) {
	return service.dependencies.WhatsAppClient.QRChannel(ctx)
}

func (service *WhatsAppService) Start(ctx context.Context) error {
	return service.dependencies.WhatsAppClient.Connect(ctx)
}

func (service *WhatsAppService) Stop(ctx context.Context) error {
	return service.dependencies.WhatsAppClient.Disconnect(ctx)
}

func (service *WhatsAppService) EnableDefaultMessageHandler() {
	service.dependencies.WhatsAppClient.RegisterMessageHandler(service.handleIncomingMessage)
}

func (service *WhatsAppService) StartMonthlyFinanceReportCron(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		var lastSentMonth time.Month

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				// Hanya jalankan pada tanggal 1
				if t.Day() == 1 && t.Month() != lastSentMonth {
					if service.dependencies.FinanceRepo == nil {
						continue
					}

					prevMonth := t.AddDate(0, -1, 0)
					reportData, err := service.dependencies.FinanceRepo.GetMonthlyReport(ctx, prevMonth.Year(), prevMonth.Month())
					if err != nil {
						log.Printf("Failed to get monthly finance report: %v", err)
						continue
					}

					for phone, totals := range reportData {
						income := totals["income"]
						expense := totals["expense"]

						diff := income - expense
						status := "Lebih banyak pemasukan"
						if diff < 0 {
							status = "Lebih banyak pengeluaran"
						} else if diff == 0 {
							status = "Seimbang"
						}

						msg := fmt.Sprintf("📊 *Laporan Keuangan Bulan %s %d*\n\n", prevMonth.Month().String(), prevMonth.Year())
						msg += fmt.Sprintf("🟢 Pemasukan: %s\n", helper.FormatRupiah(income))
						msg += fmt.Sprintf("🔴 Pengeluaran: %s\n\n", helper.FormatRupiah(expense))
						msg += fmt.Sprintf("Status: *%s*\n", status)
						msg += fmt.Sprintf("Selisih: %s\n", helper.FormatRupiah(math.Abs(diff)))

						jid := phone + "@s.whatsapp.net"
						_, err := service.dependencies.WhatsAppClient.SendText(ctx, &whatsapp.SendTextRequest{
							ChatId: jid,
							Text:   msg,
						})
						if err != nil {
							log.Printf("Failed to send monthly report to %s: %v", phone, err)
						} else {
							log.Printf("Sent monthly report to %s", phone)
						}
					}
					lastSentMonth = t.Month()
				}
			}
		}
	}()
}

func (service *WhatsAppService) handleIncomingMessage(ctx context.Context, event *whatsapp.IncomingMessageEvent) error {
	service.cacheIncomingMedia(event)
	log.Printf(
		"incoming_message message_id=%s chat_id=%s sender=%s from_me=%t body_len=%d media_type=%s quoted_message_id=%s",
		event.MessageId,
		event.ChatId,
		event.SenderJid,
		event.FromMe,
		len(strings.TrimSpace(event.Body)),
		strings.TrimSpace(event.MediaType),
		strings.TrimSpace(event.QuotedMessageId),
	)

	message := helper.NormalizeMessage(event.Body)
	if strings.HasSuffix(event.ChatId, "@broadcast") {
		log.Printf("skip_incoming_message reason=broadcast message_id=%s", event.MessageId)
		return nil
	}

	if message == "" {
		log.Printf("skip_incoming_message reason=empty_message message_id=%s", event.MessageId)
		return nil
	}

	commandName, commandArgs, explicitCommand := parseCommand(message, service.commands)
	if commandName == "" {
		log.Printf("skip_incoming_message reason=no_command message_id=%s", event.MessageId)
		return nil
	}

	handler, exists := service.commands[commandName]
	if !exists {
		if !explicitCommand {
			log.Printf("skip_incoming_message reason=not_explicit_command message_id=%s", event.MessageId)
			return nil
		}
		log.Printf("unknown_command message_id=%s command=%s", event.MessageId, commandName)
		return nil
	}

	log.Printf("execute_command_start message_id=%s command=%s args_len=%d", event.MessageId, commandName, len(commandArgs))
	reply, err := handler(ctx, event, commandArgs)
	if err != nil {
		log.Printf("execute_command_error message_id=%s command=%s error=%s", event.MessageId, commandName, helper.SanitizeUserError(err))
		errorText := "Wah error nih, silahkan coba lagi ya!"
		_, sendErr := service.dependencies.WhatsAppClient.SendText(ctx, &whatsapp.SendTextRequest{
			ChatId: event.ChatId,
			Text:   errorText,
		})
		if sendErr != nil {
			log.Printf("send_error_reply_failed message_id=%s command=%s error=%s", event.MessageId, commandName, helper.SanitizeUserError(sendErr))
			return fmt.Errorf("execute command %s: %w (failed to send error: %v)", commandName, err, sendErr)
		}
		return nil
	}
	if strings.TrimSpace(reply) == "" {
		log.Printf("execute_command_done message_id=%s command=%s reply=empty", event.MessageId, commandName)
		return nil
	}

	_, err = service.dependencies.WhatsAppClient.SendText(ctx, &whatsapp.SendTextRequest{
		ChatId: event.ChatId,
		Text:   reply,
	})
	if err != nil {
		log.Printf("send_command_reply_failed message_id=%s command=%s error=%s", event.MessageId, commandName, helper.SanitizeUserError(err))
		return fmt.Errorf("send command reply: %w", err)
	}
	log.Printf("execute_command_done message_id=%s command=%s reply_len=%d", event.MessageId, commandName, len(reply))

	return nil
}

func (service *WhatsAppService) SendImageWithCaption(
	ctx context.Context,
	chatID string,
	imageBytes []byte,
	caption string,
) (string, error) {
	response, err := service.dependencies.WhatsAppClient.SendImage(ctx, &whatsapp.SendImageRequest{
		ChatId:     chatID,
		ImageBytes: imageBytes,
		Caption:    caption,
	})
	if err != nil {
		return "", fmt.Errorf("send image with caption: %w", err)
	}

	return response.MessageId, nil
}

func (service *WhatsAppService) registerCommands() {
	service.commands["!help"] = service.handleHelp
	service.commands["!menu"] = service.handleHelp
	service.commands["!salam"] = service.handleSalam
	service.commands["!ask"] = service.handleAsk
	service.commands["!jokes"] = service.handleJokes
	service.commands["!animequote"] = service.handleAnimeQuote
	service.commands["!doa"] = service.handleDoa
	service.commands["!asmaulhusna"] = service.handleAsmaulHusna
	service.commands["!jadwalsholat"] = service.handleJadwalSholat
	service.commands["!sholat"] = service.handleJadwalSholat
	service.commands["!distro"] = service.handleDistroInfo
	service.commands["!ig"] = service.handleInstagramDownload
	service.commands["!igdl"] = service.handleInstagramDownload
	service.commands["!instagram"] = service.handleInstagramDownload
	service.commands["!tt"] = service.handleTikTokDownload
	service.commands["!ttdl"] = service.handleTikTokDownload
	service.commands["!tiktok"] = service.handleTikTokDownload
	service.commands["!tw"] = service.handleTwitterDownload
	service.commands["!twdl"] = service.handleTwitterDownload
	service.commands["!twitter"] = service.handleTwitterDownload
	service.commands["!x"] = service.handleTwitterDownload
	service.commands["!fb"] = service.handleFacebookDownload
	service.commands["!fbdl"] = service.handleFacebookDownload
	service.commands["!facebook"] = service.handleFacebookDownload
	service.commands["!editbackground"] = service.handleEditBackground
	service.commands["!editphoto"] = service.handleEditBackground
	service.commands["!ocr"] = service.handleOCR
	service.commands["!sticker"] = service.handleSticker
	service.commands["!s"] = service.handleSticker
	service.commands["!brat"] = service.handleBrat

	service.commands["!income"] = service.handleFinance
	service.commands["!expense"] = service.handleFinance
	service.commands["!checkincome"] = service.handleCheckFinance
	service.commands["!checkexpense"] = service.handleCheckFinance
	service.commands["!jadwal"] = service.handleJadwal

	service.commands["!dlonce"] = service.handleDlOnce // Download view-media-once and serve it to user
}

func (service *WhatsAppService) handleDlOnce(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	return "", nil
}

func (service *WhatsAppService) StartScheduleReminderCron(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if service.dependencies.ScheduleRepo == nil {
					continue
				}

				schedules, err := service.dependencies.ScheduleRepo.GetUpcomingSchedules(ctx)
				if err != nil {
					log.Printf("Failed to get upcoming schedules: %v", err)
					continue
				}

				now := time.Now()
				for _, sched := range schedules {
					timeDiff := sched.ScheduleTime.Sub(now)
					minutesDiff := timeDiff.Minutes()
					
					var reminderMsg string
					var reminderMin int

					if minutesDiff <= 30 && minutesDiff > 15 && !sched.Reminded30 {
						reminderMsg = fmt.Sprintf("🔔 *Reminder Jadwal*\n\nSekitar 30 menit lagi ada jadwal:\n* %s *\npada %s", sched.ScheduleName, sched.ScheduleTime.Format("15:04"))
						reminderMin = 30
					} else if minutesDiff <= 15 && minutesDiff > 5 && !sched.Reminded15 {
						reminderMsg = fmt.Sprintf("🔔 *Reminder Jadwal*\n\nSekitar 15 menit lagi ada jadwal:\n* %s *\npada %s", sched.ScheduleName, sched.ScheduleTime.Format("15:04"))
						reminderMin = 15
					} else if minutesDiff <= 5 && minutesDiff >= 0 && !sched.Reminded5 {
						reminderMsg = fmt.Sprintf("🔔 *Reminder Jadwal*\n\nSekitar 5 menit lagi (atau kurang) ada jadwal:\n* %s *\npada %s", sched.ScheduleName, sched.ScheduleTime.Format("15:04"))
						reminderMin = 5
					}

					if reminderMsg != "" {
						jid := sched.Phone + "@s.whatsapp.net"
						_, err := service.dependencies.WhatsAppClient.SendText(ctx, &whatsapp.SendTextRequest{
							ChatId: jid,
							Text:   reminderMsg,
						})
						if err != nil {
							log.Printf("Failed to send reminder to %s: %v", sched.Phone, err)
						} else {
							errUpdate := service.dependencies.ScheduleRepo.UpdateRemindedStatus(ctx, sched.ID, reminderMin)
							if errUpdate != nil {
								log.Printf("Failed to update reminder status for schedule %d: %v", sched.ID, errUpdate)
							}
						}
					}
				}
			}
		}
	}()
}

func (service *WhatsAppService) handleJadwal(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.AIClient == nil {
		return "Fitur jadwal membutuhkan AI, namun AI belum dikonfigurasi.", nil
	}
	if service.dependencies.ScheduleRepo == nil {
		return "Database jadwal belum dikonfigurasi.", nil
	}

	if strings.TrimSpace(args) == "" {
		return "Ketik format: *!jadwal <keterangan jadwal dan waktu>*\nContoh: *!jadwal bro tolong buatin reminder untuk besok jam 9 pagi mau meeting*", nil
	}

	service.sendWaitMessage(ctx, event.ChatId)

	nowStr := time.Now().Format(time.RFC3339)
	prompt := fmt.Sprintf(`Kamu adalah asisten penjadwalan. Tugasmu adalah mengekstrak nama kegiatan dan waktu dari input user.
Waktu saat ini adalah: %s
Input user: "%s"

Tentukan waktu jadwal berdasarkan input user dan waktu saat ini (ingat zona waktunya).
Balas HANYA dengan format JSON valid seperti berikut, tanpa markdown atau teks tambahan:
{
  "schedule_name": "<nama kegiatan>",
  "schedule_time": "<waktu kegiatan dalam format RFC3339>"
}`, nowStr, args)

	aiReply, err := service.dependencies.AIClient.GenerateReply(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("ai generate schedule reply: %w", err)
	}

	aiReply = strings.TrimSpace(aiReply)
	aiReply = strings.TrimPrefix(aiReply, "```json")
	aiReply = strings.TrimPrefix(aiReply, "```")
	aiReply = strings.TrimSuffix(aiReply, "```")
	aiReply = strings.TrimSpace(aiReply)

	var extractedData struct {
		ScheduleName string `json:"schedule_name"`
		ScheduleTime string `json:"schedule_time"`
	}

	if err := json.Unmarshal([]byte(aiReply), &extractedData); err != nil {
		log.Printf("Failed to unmarshal AI schedule reply: %s", aiReply)
		return "Maaf, format yang dipahami AI tidak sesuai. Coba kalimat yang lebih spesifik.", nil
	}

	parsedTime, err := time.Parse(time.RFC3339, extractedData.ScheduleTime)
	if err != nil {
		return "Gagal mengonversi waktu dari AI. Pastikan instruksimu jelas.", nil
	}

	minutesDiff := parsedTime.Sub(time.Now()).Minutes()
	if minutesDiff <= 30 {
		return "Waktu jadwal yang ditentukan haruslah >30 menit dari waktu saat ini.", nil
	}

	phone := strings.Split(event.SenderJid, "@")[0]

	record := &repositories.Schedule{
		Phone:        phone,
		ScheduleName: extractedData.ScheduleName,
		ScheduleTime: parsedTime,
	}

	if err := service.dependencies.ScheduleRepo.InsertSchedule(ctx, record); err != nil {
		return "", fmt.Errorf("insert schedule record: %w", err)
	}

	replyMsg := fmt.Sprintf("✅ *Jadwal Berhasil Disimpan*\n\nKegiatan: %s\nWaktu: %s\n\nNanti bakal diingetin 30, 15, dan 5 menit sebelum waktunya ya!",
		extractedData.ScheduleName, parsedTime.Format("02 Jan 2006 15:04"))

	return replyMsg, nil
}

func (service *WhatsAppService) handleCheckFinance(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.FinanceRepo == nil {
		return "Database keuangan belum dikonfigurasi.", nil
	}

	recordType := "income"
	if strings.HasPrefix(strings.ToLower(event.Body), "!checkexpense") {
		recordType = "expense"
	}

	now := time.Now()
	targetMonth := now.Month()
	targetYear := now.Year()

	args = strings.TrimSpace(strings.ToLower(args))
	if args != "" {
		parts := strings.Split(args, " ")

		months := map[string]time.Month{
			"januari": time.January, "jan": time.January,
			"februari": time.February, "feb": time.February,
			"maret": time.March, "mar": time.March,
			"april": time.April, "apr": time.April,
			"mei":  time.May,
			"juni": time.June, "jun": time.June,
			"juli": time.July, "jul": time.July,
			"agustus": time.August, "agu": time.August, "ags": time.August,
			"september": time.September, "sep": time.September,
			"oktober": time.October, "okt": time.October,
			"november": time.November, "nov": time.November,
			"desember": time.December, "des": time.December,
		}

		if m, ok := months[parts[0]]; ok {
			targetMonth = m
		} else if m, err := strconv.Atoi(parts[0]); err == nil && m >= 1 && m <= 12 {
			targetMonth = time.Month(m)
		}

		if len(parts) > 1 {
			if y, err := strconv.Atoi(parts[1]); err == nil && y > 2000 {
				targetYear = y
			}
		}
	}

	phone := strings.Split(event.SenderJid, "@")[0]
	records, err := service.dependencies.FinanceRepo.GetDetailedMonthlyReport(ctx, phone, targetYear, targetMonth)
	if err != nil {
		return "", fmt.Errorf("get detailed report: %w", err)
	}

	var filteredRecords []repositories.FinanceRecord
	var total float64
	for _, rec := range records {
		if rec.Type == recordType {
			filteredRecords = append(filteredRecords, rec)
			total += rec.Amount
		}
	}

	typeTitle := "Pemasukan"
	if recordType == "expense" {
		typeTitle = "Pengeluaran"
	}

	if len(filteredRecords) == 0 {
		return fmt.Sprintf("Tidak ada %s pada bulan %s %d.", typeTitle, targetMonth.String(), targetYear), nil
	}

	msg := fmt.Sprintf("📊 *Rekap %s Bulan %s %d*\n\n", typeTitle, targetMonth.String(), targetYear)
	for i, rec := range filteredRecords {
		msg += fmt.Sprintf("%d. %s - %s\n", i+1, rec.CreatedAt.Format("02/01"), helper.FormatRupiah(rec.Amount))
		msg += fmt.Sprintf("   Kategori: %s\n", rec.Category)
		msg += fmt.Sprintf("   Ket: %s\n\n", rec.Description)
	}
	msg += fmt.Sprintf("💰 *Total %s: %s*", typeTitle, helper.FormatRupiah(total))

	return msg, nil
}

func (service *WhatsAppService) handleFinance(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.AIClient == nil {
		return "Fitur pencatatan keuangan membutuhkan AI, namun AI belum dikonfigurasi.", nil
	}
	if service.dependencies.FinanceRepo == nil {
		return "Database keuangan belum dikonfigurasi.", nil
	}

	recordType := "income"
	if strings.HasPrefix(strings.ToLower(event.Body), "!expense") {
		recordType = "expense"
	}

	if strings.TrimSpace(args) == "" {
		if recordType == "income" {
			return "Ketik format: *!income <keterangan pendapatan dan jumlahnya>*\nContoh: *!income gaji bulan ini 5000000*", nil
		} else {
			return "Ketik format: *!expense <keterangan pengeluaran dan jumlahnya>*\nContoh: *!expense tadi beli bakso 15rb*", nil
		}
	}

	service.sendWaitMessage(ctx, event.ChatId)

	prompt := fmt.Sprintf(`Kamu adalah asisten pencatat keuangan. Tugasmu adalah mengekstrak jumlah uang, kategori, dan deskripsi dari input user.
Input: "%s"
Tipe pencatatan: %s

Balas HANYA dengan format JSON valid seperti berikut, tanpa markdown atau teks tambahan:
{
  "amount": <angka uang dalam bentuk number bulat, misal 15000>,
  "category": "<kategori singkat, misal makanan, gaji, transportasi>",
  "description": "<deskripsi singkat pengeluaran atau pendapatan>"
}`, args, recordType)

	aiReply, err := service.dependencies.AIClient.GenerateReply(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("ai generate finance reply: %w", err)
	}

	aiReply = strings.TrimSpace(aiReply)
	aiReply = strings.TrimPrefix(aiReply, "```json")
	aiReply = strings.TrimPrefix(aiReply, "```")
	aiReply = strings.TrimSuffix(aiReply, "```")
	aiReply = strings.TrimSpace(aiReply)

	var extractedData struct {
		Amount      float64 `json:"amount"`
		Category    string  `json:"category"`
		Description string  `json:"description"`
	}

	if err := json.Unmarshal([]byte(aiReply), &extractedData); err != nil {
		log.Printf("Failed to unmarshal AI finance reply: %s", aiReply)
		return "Maaf, format yang dipahami AI tidak sesuai. Coba kalimat yang lebih jelas.", nil
	}

	if extractedData.Amount <= 0 {
		return "Gagal mendeteksi jumlah uang yang valid. Pastikan kamu menyebutkan nominalnya.", nil
	}

	phone := strings.Split(event.SenderJid, "@")[0]

	record := &repositories.FinanceRecord{
		Phone:       phone,
		Type:        recordType,
		Amount:      extractedData.Amount,
		Category:    extractedData.Category,
		Description: extractedData.Description,
		CreatedAt:   time.Now(),
	}

	if err := service.dependencies.FinanceRepo.InsertRecord(ctx, record); err != nil {
		return "", fmt.Errorf("insert finance record: %w", err)
	}

	amountStr := helper.FormatRupiah(extractedData.Amount)

	replyMsg := fmt.Sprintf("✅ *Pencatatan Berhasil*\n\nTipe: %s\nNominal: %s\nKategori: %s\nDeskripsi: %s",
		strings.Title(recordType), amountStr, extractedData.Category, extractedData.Description)

	return replyMsg, nil
}

func (service *WhatsAppService) handleHelp(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	command := "Ryuko Matoi - github.com/haikelz*\n\n" +
		"*Umum*\n" +
		"!salam\n" +
		"!info\n" +
		"!ask <pertanyaan>\n" +
		"!jokes\n" +
		"!animequote\n\n" +
		"*Islami*\n" +
		"!doa [semua|keyword]\n" +
		"!asmaulhusna [nomor|nama latin]\n" +
		"!jadwalsholat <kota>\n\n" +
		"*Media Downloader*\n" +
		"!igdl <link_instagram>\n" +
		"!ttdl <link_tiktok>\n" +
		"!twdl <link_twitter>\n" +
		"!fbdl <link_facebook>\n" +
		"*Tools*\n" +
		"!editbackground <warna>\n" +
		"!ocr\n" +
		"!sticker\n" +
		"!brat <teks>\n\n" +
		"*Keuangan*\n" +
		"!income <nominal dan deskripsi>\n" +
		"!expense <nominal dan deskripsi>\n"

	if len(assets.HelpThumbnailPNG) == 0 {
		return "", fmt.Errorf("help thumbnail asset is empty")
	}

	_, err := service.SendImageWithCaption(ctx, event.ChatId, assets.HelpThumbnailPNG, command)
	if err != nil {
		return "", fmt.Errorf("send help image: %w", err)
	}

	return "", nil
}

func (service *WhatsAppService) handleSalam(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	service.sendWaitMessage(ctx, event.ChatId)
	return "Assalamu'alaikum", nil
}

func (service *WhatsAppService) handleAsk(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	question := strings.TrimSpace(args)
	if question == "" {
		return "Ini adalah perintah untuk mendapatkan jawaban dari AI. Cukup ketik *!ask <your_pertanyaan>*", nil
	}
	if service.dependencies.AIClient == nil {
		return "Fitur !ask belum aktif: AI client belum diintegrasikan.", nil
	}
	service.sendWaitMessage(ctx, event.ChatId)

	mediaData, mediaType, mediaMime := service.resolveEventOrQuotedMedia(event)
	if len(mediaData) > 0 && mediaType == "image" {
		reply, err := service.dependencies.AIClient.GenerateReplyWithImage(ctx, question, mediaData, mediaMime)
		if err != nil {
			return "", fmt.Errorf("generate ai reply with image: %w", err)
		}
		if strings.TrimSpace(reply) == "" {
			return "Maaf, saya tidak tau jawabannya", nil
		}
		return reply, nil
	}

	reply, err := service.dependencies.AIClient.GenerateReply(ctx, question)
	if err != nil {
		return "", fmt.Errorf("generate ai reply: %w", err)
	}
	if strings.TrimSpace(reply) == "" {
		return "Maaf, saya tidak tau jawabannya", nil
	}

	return reply, nil
}

func (service *WhatsAppService) handleJokes(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.HTTPClient == nil {
		return "HTTP client belum di-wire.", nil
	}
	if service.dependencies.Config.Api.JokesUrl == "" {
		return "JOKES_API_URL belum dikonfigurasi.", nil
	}
	service.sendWaitMessage(ctx, event.ChatId)

	base := strings.TrimRight(service.dependencies.Config.Api.JokesUrl, "/")
	textPayload, err := service.dependencies.HTTPClient.Get(ctx, base+"/api/text/random")
	if err != nil {
		return "", fmt.Errorf("fetch random joke text: %w", err)
	}
	imagePayload, err := service.dependencies.HTTPClient.Get(ctx, base+"/api/image/random")
	if err != nil {
		return "", fmt.Errorf("fetch random joke image: %w", err)
	}

	textParsed, err := helper.DecodeJSON(textPayload)
	if err != nil {
		return "Wah error nih, silahkan coba lagi ya!", nil
	}
	imageParsed, err := helper.DecodeJSON(imagePayload)
	if err != nil {
		return "Wah error nih, silahkan coba lagi ya!", nil
	}

	textCandidate := helper.FirstNonEmpty(
		helper.ExtractStringByPath(textParsed, "data"),
		helper.ExtractStringByPath(textParsed, "data", "text"),
		helper.ExtractStringByPath(textParsed, "text"),
	)
	imageURL := helper.FirstNonEmpty(
		helper.ExtractStringByPath(imageParsed, "data", "url"),
		helper.ExtractStringByPath(imageParsed, "data"),
		helper.ExtractStringByPath(imageParsed, "url"),
	)

	if textCandidate == "" || imageURL == "" {
		return "Jokes tidak tersedia saat ini.", nil
	}

	imageBytes, err := service.fetchImage(ctx, imageURL)
	if err != nil {
		return "", fmt.Errorf("download joke image: %w", err)
	}

	_, err = service.dependencies.WhatsAppClient.SendImage(ctx, &whatsapp.SendImageRequest{
		ChatId:     event.ChatId,
		ImageBytes: imageBytes,
		Caption:    textCandidate,
	})
	if err != nil {
		return "", fmt.Errorf("send joke image: %w", err)
	}

	return "", nil
}

func (service *WhatsAppService) handleAnimeQuote(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.HTTPClient == nil {
		return "HTTP client belum di-wire.", nil
	}
	if service.dependencies.Config.Api.AnimeQuoteUrl == "" {
		return "ANIME_QUOTE_API_URL belum dikonfigurasi.", nil
	}

	base := strings.TrimRight(service.dependencies.Config.Api.AnimeQuoteUrl, "/")
	animeQuery := strings.ToLower(strings.TrimSpace(args))
	service.sendWaitMessage(ctx, event.ChatId)

	var endpoint string
	if animeQuery != "" {
		endpoint = fmt.Sprintf("%s/api/getbyanime?anime=%s&page=1", base, url.QueryEscape(animeQuery))
	} else {
		endpoint = fmt.Sprintf("%s/api/getrandom", base)
	}

	payload, err := service.dependencies.HTTPClient.Get(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("fetch anime quote: %w", err)
	}
	if looksLikeHTML(payload) {
		return "Anime quote Api mengembalikan HTML, bukan JSON. Periksa ANIME_QUOTE_API_URL.", nil
	}

	parsed, err := helper.DecodeJSON(payload)
	if err != nil {
		return "Format response anime quote tidak valid.", nil
	}

	resultList := extractResultList(parsed)
	if len(resultList) == 0 {
		return "Anime quote tidak tersedia saat ini.", nil
	}

	lines := make([]string, 0, len(resultList))
	for _, item := range resultList {
		indo := strings.TrimSpace(helper.StringValue(item["indo"]))
		if indo == "" {
			indo = strings.TrimSpace(helper.StringValue(item["quote"]))
		}
		if indo == "" {
			continue
		}

		if animeQuery != "" {
			lines = append(lines, "- "+indo)
			continue
		}

		animeName := strings.TrimSpace(helper.StringValue(item["anime"]))
		if animeName != "" {
			lines = append(lines, fmt.Sprintf("*%s*\n- %s", animeName, indo))
			continue
		}
		lines = append(lines, "- "+indo)
	}

	if len(lines) == 0 {
		return "Anime quote tidak tersedia saat ini.", nil
	}

	return strings.Join(lines, "\n\n"), nil
}

func (service *WhatsAppService) handleDoa(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.HTTPClient == nil {
		return "HTTP client belum di-wire.", nil
	}
	if service.dependencies.Config.Api.DoaUrl == "" {
		return "DOA_API_URL belum dikonfigurasi.", nil
	}

	query := strings.TrimSpace(args)
	base := strings.TrimRight(service.dependencies.Config.Api.DoaUrl, "/")
	service.sendWaitMessage(ctx, event.ChatId)

	if strings.EqualFold(query, "info") {
		return "Ini adalah perintah untuk mendapatkan Do'a secara spesifik, maupun secara random. Ketik *!doa* untuk mendapatkan hasil random, dan *!doa semua* untuk mendapatkan semua", nil
	}

	if query == "" {
		payload, err := service.dependencies.HTTPClient.Get(ctx, base+"/api/doa/v1/random")
		if err != nil {
			return "", fmt.Errorf("fetch random doa: %w", err)
		}
		parsed, err := helper.DecodeJSON(payload)
		if err != nil {
			return "Wah error nih, silahkan coba lagi ya!", nil
		}
		items := extractDoaList(parsed)
		if len(items) == 0 {
			return "Data doa tidak tersedia.", nil
		}
		lines := make([]string, 0, len(items))
		for _, item := range items {
			doaName := helper.FirstNonEmpty(helper.StringValue(item["doa"]), helper.StringValue(item["nama"]))
			ayat := helper.StringValue(item["ayat"])
			arti := helper.StringValue(item["artinya"])
			lines = append(lines, fmt.Sprintf("*%s*:\n\n%s\nArtinya: %s", doaName, ayat, arti))
		}
		return strings.Join(lines, "\n"), nil
	}

	if strings.EqualFold(query, "semua") {
		payload, err := service.dependencies.HTTPClient.Get(ctx, base+"/api")
		if err != nil {
			return "", fmt.Errorf("fetch all doa: %w", err)
		}
		parsed, err := helper.DecodeJSON(payload)
		if err != nil {
			return "Wah error nih, silahkan coba lagi ya!", nil
		}
		items := extractDoaList(parsed)
		if len(items) == 0 {
			return "Data doa tidak tersedia.", nil
		}
		lines := make([]string, 0, len(items))
		for _, item := range items {
			doaName := helper.FirstNonEmpty(helper.StringValue(item["doa"]), helper.StringValue(item["nama"]))
			ayat := helper.StringValue(item["ayat"])
			arti := helper.StringValue(item["artinya"])
			lines = append(lines, fmt.Sprintf("*%s*:\n%s\nArtinya: %s", doaName, ayat, arti))
		}
		return strings.Join(lines, "\n\n"), nil
	}

	payload, err := service.dependencies.HTTPClient.Get(ctx, base+"/api/doa/"+url.PathEscape(strings.ToLower(query)))
	if err != nil {
		return "", fmt.Errorf("fetch doa by name: %w", err)
	}
	parsed, err := helper.DecodeJSON(payload)
	if err != nil {
		return "Wah error nih, silahkan coba lagi ya!", nil
	}
	item := helper.PickFirstMap(parsed)
	if item == nil {
		return "Data doa tidak ditemukan.", nil
	}

	doaName := helper.FirstNonEmpty(helper.StringValue(item["doa"]), helper.StringValue(item["nama"]))
	ayat := helper.StringValue(item["ayat"])
	arti := helper.StringValue(item["artinya"])
	return fmt.Sprintf("*%s*:\n\n%s\nArtinya: %s", doaName, ayat, arti), nil
}

func (service *WhatsAppService) handleAsmaulHusna(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.HTTPClient == nil {
		return "HTTP client belum di-wire.", nil
	}
	if service.dependencies.Config.Api.AsmaulHusnaUrl == "" {
		return "ASMAUL_HUSNA_API_URL belum dikonfigurasi.", nil
	}

	query := strings.TrimSpace(args)
	base := strings.TrimRight(service.dependencies.Config.Api.AsmaulHusnaUrl, "/")
	service.sendWaitMessage(ctx, event.ChatId)

	if strings.EqualFold(query, "info") {
		return "Ini adalah perintah untuk mendapatkan Asmaul Husna.\n- Masukkan nomor urut untuk mendapatkan Asma'ul Husna berdasarkan nomor urut.\n- Masukkan nama latin untuk mendapatkan Asma'ul Husna berdasarkan nama latin.\n- Biarkan kosong untuk mendapatkan semua daftar Asma'ul Husna.", nil
	}

	if query == "" {
		payload, err := service.dependencies.HTTPClient.Get(ctx, base+"/api/all")
		if err != nil {
			return "", fmt.Errorf("fetch all asmaul husna: %w", err)
		}
		parsed, err := helper.DecodeJSON(payload)
		if err != nil {
			return "Wah error nih, silahkan coba lagi ya!", nil
		}
		items := extractAsmaulHusnaList(parsed)
		if len(items) == 0 {
			return "Data Asmaul Husna tidak ditemukan.", nil
		}
		lines := make([]string, 0, len(items))
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("%s - %s\n%s\n%s",
				helper.FirstNonEmpty(helper.StringValue(item["urutan"]), helper.StringValue(item["index"])),
				helper.FirstNonEmpty(helper.StringValue(item["arab"]), helper.StringValue(item["asma"])),
				helper.StringValue(item["latin"]),
				helper.StringValue(item["arti"]),
			))
		}
		return strings.Join(lines, "\n"), nil
	}

	var endpoint string
	if _, err := strconv.Atoi(query); err == nil {
		endpoint = fmt.Sprintf("%s/api/%s", base, url.PathEscape(query))
	} else {
		endpoint = fmt.Sprintf("%s/api/latin/%s", base, helper.SlugifyBasic(query))
	}

	payload, err := service.dependencies.HTTPClient.Get(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("fetch asmaul husna: %w", err)
	}
	parsed, err := helper.DecodeJSON(payload)
	if err != nil {
		return "Wah error nih, silahkan coba lagi ya!", nil
	}
	item := helper.PickFirstMap(parsed)
	if item == nil {
		return "Data Asmaul Husna tidak ditemukan.", nil
	}

	return fmt.Sprintf("%s - %s\n%s\n%s",
		helper.FirstNonEmpty(helper.StringValue(item["urutan"]), helper.StringValue(item["index"])),
		helper.FirstNonEmpty(helper.StringValue(item["arab"]), helper.StringValue(item["asma"])),
		helper.StringValue(item["latin"]),
		helper.FirstNonEmpty(helper.StringValue(item["arti"]), helper.StringValue(item["translation"])),
	), nil
}

func (service *WhatsAppService) handleJadwalSholat(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.HTTPClient == nil {
		return "HTTP client belum di-wire.", nil
	}
	if service.dependencies.Config.Api.QuranUrl == "" {
		return "QURAN_API_URL belum dikonfigurasi.", nil
	}

	city := strings.TrimSpace(args)
	service.sendWaitMessage(ctx, event.ChatId)
	if city == "" {
		return "Ini adalah perintah untuk mendapatkan jadwal sholat sesuai dengan nama daerah yang dimasukkan. Cukup ketik *!jadwalsholat <your_daerah>*", nil
	}
	if len([]rune(city)) <= 2 {
		return "Maaf, panjang karakter daerah yang dimasukkan tidak boleh kurang dari atau sama dengan 2!", nil
	}

	base := strings.TrimRight(service.dependencies.Config.Api.QuranUrl, "/")
	searchURL := fmt.Sprintf("%s/v2/sholat/kota/cari/%s", base, url.PathEscape(city))
	searchPayload, err := service.dependencies.HTTPClient.Get(ctx, searchURL)
	if err != nil {
		return "", fmt.Errorf("search city for prayer schedule: %w", err)
	}

	searchParsed, err := helper.DecodeJSON(searchPayload)
	if err != nil {
		return "Format response jadwal sholat tidak valid.", nil
	}

	cityID, cityName := extractCity(searchParsed)
	if cityID == "" {
		return "Kota tidak ditemukan. Coba gunakan nama kota lain.", nil
	}

	now := time.Now()
	scheduleURL := fmt.Sprintf("%s/v2/sholat/jadwal/%s/%d/%d/%d", base, cityID, now.Year(), int(now.Month()), now.Day())
	schedulePayload, err := service.dependencies.HTTPClient.Get(ctx, scheduleURL)
	if err != nil {
		return "", fmt.Errorf("fetch prayer schedule: %w", err)
	}

	scheduleParsed, err := helper.DecodeJSON(schedulePayload)
	if err != nil {
		return "Format data jadwal sholat tidak valid.", nil
	}

	jadwal := extractScheduleMap(scheduleParsed)
	if jadwal == nil {
		return "Jadwal sholat tidak tersedia.", nil
	}

	tanggal := helper.FirstNonEmpty(helper.StringValue(jadwal["tanggal"]), helper.StringValue(jadwal["date"]))
	imsak := helper.FirstNonEmpty(helper.StringValue(jadwal["imsak"]))
	subuh := helper.FirstNonEmpty(helper.StringValue(jadwal["subuh"]))
	dzuhur := helper.FirstNonEmpty(helper.StringValue(jadwal["dzuhur"]))
	ashar := helper.FirstNonEmpty(helper.StringValue(jadwal["ashar"]))
	maghrib := helper.FirstNonEmpty(helper.StringValue(jadwal["maghrib"]))
	isya := helper.FirstNonEmpty(helper.StringValue(jadwal["isya"]))

	indonesianDate := time.Now().Format("02 January 2006")
	lines := []string{fmt.Sprintf("*Jadwal Sholat hari ini, %s di Kota %s*", indonesianDate, strings.Title(helper.FirstNonEmpty(cityName, city)))}
	if tanggal != "" {
		lines = append(lines, "Tanggal: "+tanggal)
	}
	if imsak != "" {
		lines = append(lines, "Imsak: "+imsak)
	}
	if subuh != "" {
		lines = append(lines, "Subuh: "+subuh)
	}
	if dzuhur != "" {
		lines = append(lines, "Dzuhur: "+dzuhur)
	}
	if ashar != "" {
		lines = append(lines, "Ashar: "+ashar)
	}
	if maghrib != "" {
		lines = append(lines, "Maghrib: "+maghrib)
	}
	if isya != "" {
		lines = append(lines, "Isya: "+isya)
	}

	return strings.Join(lines, "\n"), nil
}

func (service *WhatsAppService) handleDistroInfo(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.HTTPClient == nil {
		return "HTTP client belum di-wire.", nil
	}
	if strings.TrimSpace(service.dependencies.Config.Api.DistroInfoUrl) == "" {
		return "Fitur distro tidak aktif (DISTRO_INFO_API_URL kosong / deprecated).", nil
	}
	service.sendWaitMessage(ctx, event.ChatId)

	distro := strings.TrimSpace(args)
	if distro == "" {
		return "Ini adalah perintah untuk mencari informasi tentang Linux Distro yang diinginkan. Cukup ketik *!distro <nama_distro>*", nil
	}
	if len([]rune(distro)) <= 2 {
		return "Maaf, panjang karakter nama distro yang dimasukkan tidak boleh kurang dari atau sama dengan 2!", nil
	}

	endpoint := fmt.Sprintf("%s/api/v2/distributions/%s", strings.TrimRight(strings.TrimSpace(service.dependencies.Config.Api.DistroInfoUrl), "/"), url.PathEscape(distro))
	payload, err := service.dependencies.HTTPClient.Get(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("fetch distro info: %w", err)
	}

	parsed, err := helper.DecodeJSON(payload)
	if err != nil {
		return strings.TrimSpace(string(payload)), nil
	}

	root, ok := parsed.(map[string]any)
	if !ok {
		return compactJSON(parsed), nil
	}
	about := helper.FirstNonEmpty(helper.StringValue(root["about"]), helper.ExtractStringByPath(root, "data", "about"))
	architectures := extractStringList(root, "architectures")
	desktopEnvs := extractStringList(root, "desktop_environments")
	documentations := extractStringList(root, "documentations")
	downloadMirrors := extractStringList(root, "download_mirrors")
	distroName := helper.FirstNonEmpty(helper.StringValue(root["distribution"]), distro)

	return fmt.Sprintf(`*About:*
%s

*Architectures:*
%s

*Available Desktop Environment:*
%s

*Documentation:*
%s

*Download %s:*
%s`,
		about,
		helper.FormatBulletList(architectures),
		helper.FormatBulletList(desktopEnvs),
		helper.FormatBulletList(documentations),
		distroName,
		helper.FormatBulletList(downloadMirrors),
	), nil
}

func (service *WhatsAppService) handleInstagramDownload(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.InstagramClient == nil {
		return "Fitur Instagram downloader belum aktif.", nil
	}

	instagramURL := strings.TrimSpace(args)
	if instagramURL == "" {
		return "Gunakan format *!igdl <link_instagram>*. Link yang didukung: post, reel, atau IGTV publik.", nil
	}

	service.sendWaitMessage(ctx, event.ChatId)

	mediaItems, err := service.dependencies.InstagramClient.Download(ctx, instagramURL)
	if err != nil {
		logDownloaderFailure("instagram", instagramURL, err)
		switch {
		case errors.Is(err, instagram.ErrYtDlpNotFound):
			return "yt-dlp belum terinstall. Jalankan: pip install yt-dlp", nil
		case errors.Is(err, instagram.ErrInvalidInstagramURL):
			return "Link Instagram tidak valid. Gunakan link post, reel, atau IGTV publik.", nil
		case errors.Is(err, instagram.ErrNoMediaFound):
			return "Media Instagram tidak ditemukan. Pastikan post bersifat publik.", nil
		default:
			return "", fmt.Errorf("download instagram media: %w", err)
		}
	}

	for index, mediaItem := range mediaItems {
		caption := ""
		if len(mediaItems) > 1 {
			caption = fmt.Sprintf("Instagram media %d/%d", index+1, len(mediaItems))
		}

		switch mediaItem.Type {
		case "image":
			_, err = service.dependencies.WhatsAppClient.SendImage(ctx, &whatsapp.SendImageRequest{
				ChatId:     event.ChatId,
				ImageBytes: mediaItem.Data,
				Caption:    caption,
			})
		case "video":
			_, err = service.dependencies.WhatsAppClient.SendVideo(ctx, &whatsapp.SendVideoRequest{
				ChatId:     event.ChatId,
				VideoBytes: mediaItem.Data,
				MimeType:   mediaItem.MimeType,
				Caption:    caption,
			})
		default:
			err = fmt.Errorf("unsupported instagram media type: %s", mediaItem.MimeType)
		}
		if err != nil {
			return "", fmt.Errorf("send instagram media: %w", err)
		}
	}

	return "", nil
}

func (service *WhatsAppService) handleTikTokDownload(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.TikTokClient == nil {
		return "Fitur TikTok downloader belum aktif.", nil
	}

	link := strings.TrimSpace(args)
	if link == "" {
		return "Gunakan format *!ttdl <link_tiktok>*.", nil
	}

	service.sendWaitMessage(ctx, event.ChatId)

	mediaItems, err := service.dependencies.TikTokClient.Download(ctx, link)
	if err != nil {
		logDownloaderFailure("tiktok", link, err)
		switch {
		case errors.Is(err, media_downloader.ErrYtDlpNotFound):
			return "yt-dlp belum terinstall. Jalankan: pip install yt-dlp", nil
		case errors.Is(err, media_downloader.ErrInvalidURL):
			return "Link TikTok tidak valid.", nil
		case errors.Is(err, media_downloader.ErrNoMediaFound):
			return "Media TikTok tidak ditemukan.", nil
		default:
			return "", fmt.Errorf("download tiktok media: %w", err)
		}
	}

	return "", service.sendMediaItems(ctx, event.ChatId, mediaItems, "TikTok")
}

func (service *WhatsAppService) handleTwitterDownload(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.TwitterClient == nil {
		return "Fitur Twitter/X downloader belum aktif.", nil
	}

	link := strings.TrimSpace(args)
	if link == "" {
		return "Gunakan format *!twdl <link_twitter>*.", nil
	}

	service.sendWaitMessage(ctx, event.ChatId)

	mediaItems, err := service.dependencies.TwitterClient.Download(ctx, link)
	if err != nil {
		logDownloaderFailure("twitter", link, err)
		switch {
		case errors.Is(err, media_downloader.ErrYtDlpNotFound):
			return "yt-dlp belum terinstall. Jalankan: pip install yt-dlp", nil
		case errors.Is(err, media_downloader.ErrInvalidURL):
			return "Link Twitter/X tidak valid. Gunakan link yang mengandung /status/.", nil
		case errors.Is(err, media_downloader.ErrNoMediaFound):
			return "Media Twitter/X tidak ditemukan.", nil
		default:
			return "", fmt.Errorf("download twitter media: %w", err)
		}
	}

	return "", service.sendMediaItems(ctx, event.ChatId, mediaItems, "Twitter")
}

func (service *WhatsAppService) handleFacebookDownload(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.FacebookClient == nil {
		return "Fitur Facebook downloader belum aktif.", nil
	}

	link := strings.TrimSpace(args)
	if link == "" {
		return "Gunakan format *!fbdl <link_facebook>*.", nil
	}

	service.sendWaitMessage(ctx, event.ChatId)

	mediaItems, err := service.dependencies.FacebookClient.Download(ctx, link)
	if err != nil {
		logDownloaderFailure("facebook", link, err)
		switch {
		case errors.Is(err, media_downloader.ErrYtDlpNotFound):
			return "yt-dlp belum terinstall. Jalankan: pip install yt-dlp", nil
		case errors.Is(err, media_downloader.ErrInvalidURL):
			return "Link Facebook tidak valid.", nil
		case errors.Is(err, media_downloader.ErrNoMediaFound):
			return "Media Facebook tidak ditemukan atau butuh login. Coba link lain / set YTDLP_COOKIES_FILE.", nil
		default:
			return "", fmt.Errorf("download facebook media: %w", err)
		}
	}

	if err := service.sendMediaItems(ctx, event.ChatId, mediaItems, "Facebook"); err != nil {
		logDownloaderFailure("facebook-send", link, err)
		return "Media Facebook berhasil diunduh, tapi gagal dikirim ke WhatsApp. Coba link lain atau video dengan durasi/ukuran lebih kecil.", nil
	}

	return "", nil
}

func (service *WhatsAppService) sendMediaItems(ctx context.Context, chatId string, mediaItems []media_downloader.Media, platform string) error {
	for index, mediaItem := range mediaItems {
		caption := ""
		if len(mediaItems) > 1 {
			caption = fmt.Sprintf("%s media %d/%d", platform, index+1, len(mediaItems))
		}

		var err error
		switch mediaItem.Type {
		case "image":
			_, err = service.dependencies.WhatsAppClient.SendImage(ctx, &whatsapp.SendImageRequest{
				ChatId:     chatId,
				ImageBytes: mediaItem.Data,
				Caption:    caption,
			})
		case "video":
			shouldForceNormalize := strings.EqualFold(platform, "Facebook")
			videoBytes, videoMime, normalizeErr := normalizeVideoForWhatsApp(mediaItem.Data, mediaItem.MimeType, shouldForceNormalize)
			if normalizeErr == nil && len(videoBytes) > 0 {
				mediaItem.Data = videoBytes
				mediaItem.MimeType = videoMime
			}

			_, err = service.dependencies.WhatsAppClient.SendVideo(ctx, &whatsapp.SendVideoRequest{
				ChatId:     chatId,
				VideoBytes: mediaItem.Data,
				MimeType:   mediaItem.MimeType,
				Caption:    caption,
			})
		default:
			err = fmt.Errorf("unsupported %s media type: %s", platform, mediaItem.MimeType)
		}
		if err != nil {
			return fmt.Errorf("send %s media: %w", platform, err)
		}
	}
	return nil
}

func (service *WhatsAppService) handleEditBackground(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.Config.Media.RemoveBgApiKey == "" {
		return "REMOVE_BG_API_KEY belum diatur.", nil
	}
	if service.dependencies.Config.Api.RemoveBgUrl == "" {
		return "REMOVE_BG_API_URL belum diatur.", nil
	}

	color := strings.ToLower(strings.TrimSpace(args))
	service.sendWaitMessage(ctx, event.ChatId)
	if color == "" {
		return "Ini adalah perintah untuk mengubah warna background dari sebuah foto. Nama warna yang dimasukkan harus dalam bahasa Inggris.  Contoh: *!editphoto red*", nil
	}
	if len([]rune(color)) <= 2 {
		return "Maaf, panjang karakter yang dimasukkan tidak boleh kurang dari atau sama dengan 2!", nil
	}

	mediaData, mediaType, _ := service.resolveEventOrQuotedMedia(event)
	if len(mediaData) == 0 {
		return "Silakan kirim/reply gambar lalu gunakan *!editbackground <warna>*", nil
	}
	if mediaType != "image" {
		return "Maaf! Sepertinya file yang kamu berikan bukan gambar", nil
	}

	outputImage, err := service.callRemoveBG(ctx, mediaData, color)
	if err != nil {
		return "", fmt.Errorf("remove bg request: %w", err)
	}

	_, err = service.SendImageWithCaption(ctx, event.ChatId, outputImage, "Berhasil edit background")
	if err != nil {
		return "", fmt.Errorf("send edited image: %w", err)
	}

	return "", nil
}

func (service *WhatsAppService) handleOCR(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	if service.dependencies.OCRClient == nil {
		return "Fitur OCR belum aktif: OCR client belum di-wire.", nil
	}

	service.sendWaitMessage(ctx, event.ChatId)

	imageBytes, mediaType, mediaMime := service.resolveEventOrQuotedMedia(event)
	if len(imageBytes) == 0 {
		return "Silakan kirim/reply gambar lalu gunakan *!ocr*", nil
	}
	if mediaType != "image" {
		primaryType := strings.Split(helper.FirstNonEmpty(mediaMime, "unknown/unknown"), "/")[0]
		return fmt.Sprintf("*Format file yang anda masukkan salah!* Silahkan masukkan file berupa gambar. Format file yang anda masukkan: %s", primaryType), nil
	}

	text, err := service.dependencies.OCRClient.ExtractText(ctx, imageBytes)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "binary ocr") || strings.Contains(strings.ToLower(err.Error()), "tesseract") {
			return "OCR belum bisa dipakai karena binary tesseract belum tersedia di environment.", nil
		}
		return "", fmt.Errorf("extract ocr text: %w", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "Teks tidak terdeteksi.", nil
	}

	return text, nil
}

func (service *WhatsAppService) handleSticker(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	service.sendWaitMessage(ctx, event.ChatId)

	var mediaBytes []byte
	mediaType := strings.TrimSpace(event.MediaType)
	mediaMime := strings.TrimSpace(event.MediaMime)

	if len(event.MediaData) > 0 && (event.MediaType == "image" || event.MediaType == "video") {
		mediaBytes = event.MediaData
	} else if cached, ok := service.getCachedMedia(event.QuotedMessageId); ok {
		mediaBytes = cached.MediaData
		mediaType = cached.MediaType
		mediaMime = cached.MediaMime
	} else {
		mediaURL := strings.TrimSpace(args)
		if mediaURL == "" {
			return "Silakan kirim/reply gambar atau video lalu gunakan *!sticker*", nil
		}

		var err error
		mediaBytes, err = service.fetchImage(ctx, mediaURL)
		if err != nil {
			return "", fmt.Errorf("download media for sticker: %w", err)
		}
	}

	kind := detectStickerMediaKind(mediaType, mediaMime, mediaBytes)

	switch kind {
	case "static":
		stickerWebP, err := buildStaticSticker(mediaBytes)
		if err != nil {
			return "", fmt.Errorf("build static sticker: %w", err)
		}
		_, err = service.dependencies.WhatsAppClient.SendSticker(ctx, &whatsapp.SendStickerRequest{
			ChatId:      event.ChatId,
			StickerWebp: stickerWebP,
			Width:       512,
			Height:      512,
		})
		if err != nil {
			return "", fmt.Errorf("send static sticker: %w", err)
		}
		return "", nil
	case "animated":
		ffmpegBinary, err := resolveBinaryPath("ffmpeg")
		if err != nil {
			return "", err
		}

		stickerWebP, stickerWidth, stickerHeight, err := buildAnimatedStickerWithFFmpeg(ctx, ffmpegBinary, mediaBytes)
		if err != nil {
			return "", err
		}

		_, err = service.dependencies.WhatsAppClient.SendSticker(ctx, &whatsapp.SendStickerRequest{
			ChatId:      event.ChatId,
			StickerWebp: stickerWebP,
			Width:       stickerWidth,
			Height:      stickerHeight,
		})
		if err != nil {
			return "", fmt.Errorf("send animated sticker: %w", err)
		}
		return "", nil
	default:
		primaryType := strings.Split(helper.FirstNonEmpty(mediaMime, "unknown/unknown"), "/")[0]
		return fmt.Sprintf("*Format file yang anda masukkan salah!* Silahkan masukkan file berupa gambar/video. Format file yang anda masukkan: %s", primaryType), nil
	}
}

func (service *WhatsAppService) handleBrat(ctx context.Context, event *whatsapp.IncomingMessageEvent, args string) (string, error) {
	text := strings.TrimSpace(args)
	if text == "" {
		return "Gunakan format *!brat <teks>*.", nil
	}

	service.sendWaitMessage(ctx, event.ChatId)

	stickerWebP, err := buildBratSticker(text)
	if err != nil {
		return "", fmt.Errorf("build brat sticker: %w", err)
	}

	_, err = service.dependencies.WhatsAppClient.SendSticker(ctx, &whatsapp.SendStickerRequest{
		ChatId:      event.ChatId,
		StickerWebp: stickerWebP,
		Width:       512,
		Height:      512,
	})
	if err != nil {
		return "", fmt.Errorf("send brat sticker: %w", err)
	}

	return "", nil
}

func buildAnimatedStickerWithFFmpeg(ctx context.Context, ffmpegBinary string, mediaBytes []byte) ([]byte, uint32, uint32, error) {
	tempDir, err := os.MkdirTemp("", "ryuko-sticker-*")
	if err != nil {
		return nil, 0, 0, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input.bin")
	outputPath := filepath.Join(tempDir, "output.webp")

	if err := os.WriteFile(inputPath, mediaBytes, 0o600); err != nil {
		return nil, 0, 0, fmt.Errorf("write temp input file: %w", err)
	}

	scaledWidth, scaledHeight, err := probeStickerDimensions(ctx, ffmpegBinary, inputPath)
	if err != nil {
		return nil, 0, 0, err
	}

	ffmpegArgs := []string{
		"-y",
		"-i", inputPath,
		"-vf", fmt.Sprintf("fps=15,scale=%d:%d:flags=lanczos,format=rgba", scaledWidth, scaledHeight),
		"-loop", "0",
		"-ss", "0",
		"-t", "6",
		"-an",
		"-vsync", "0",
		outputPath,
	}
	if err := runCommand(ctx, ffmpegBinary, ffmpegArgs...); err != nil {
		return nil, 0, 0, fmt.Errorf("convert sticker with ffmpeg: %w", err)
	}

	stickerBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("read sticker output: %w", err)
	}
	if len(stickerBytes) == 0 {
		return nil, 0, 0, fmt.Errorf("sticker output is empty")
	}

	return stickerBytes, scaledWidth, scaledHeight, nil
}

func probeStickerDimensions(ctx context.Context, ffmpegBinary string, inputPath string) (uint32, uint32, error) {
	ffprobeBinary, err := resolveSiblingBinary(ffmpegBinary, "ffprobe")
	if err != nil {
		return 0, 0, err
	}

	command := exec.CommandContext(
		ctx,
		ffprobeBinary,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0:s=x",
		inputPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return 0, 0, fmt.Errorf("probe sticker dimensions: %w", err)
		}
		return 0, 0, fmt.Errorf("probe sticker dimensions: %w: %s", err, trimmed)
	}

	width, height, err := parseStickerDimensions(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, 0, err
	}

	scaledWidth, scaledHeight := scaleStickerDimensions(width, height)
	return scaledWidth, scaledHeight, nil
}

func parseStickerDimensions(value string) (uint32, uint32, error) {
	parts := strings.Split(strings.TrimSpace(value), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid sticker dimensions output: %q", value)
	}

	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("parse sticker width: %w", err)
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("parse sticker height: %w", err)
	}
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("invalid sticker dimensions: %dx%d", width, height)
	}

	return uint32(width), uint32(height), nil
}

func scaleStickerDimensions(width uint32, height uint32) (uint32, uint32) {
	const maxStickerSize = 512

	scaledWidth := float64(width)
	scaledHeight := float64(height)
	scale := 1.0

	if width >= height && width > maxStickerSize {
		scale = float64(maxStickerSize) / float64(width)
	}
	if height > width && height > maxStickerSize {
		scale = float64(maxStickerSize) / float64(height)
	}

	if scale < 1.0 {
		scaledWidth *= scale
		scaledHeight *= scale
	}

	resultWidth := uint32(math.Round(scaledWidth))
	resultHeight := uint32(math.Round(scaledHeight))
	if resultWidth == 0 {
		resultWidth = 1
	}
	if resultHeight == 0 {
		resultHeight = 1
	}

	return resultWidth, resultHeight
}

func resolveSiblingBinary(binaryPath string, siblingName string) (string, error) {
	if siblingName == "" {
		return "", fmt.Errorf("sibling binary name is empty")
	}

	siblingPath := filepath.Join(filepath.Dir(binaryPath), siblingName)
	if _, err := os.Stat(siblingPath); err == nil {
		return siblingPath, nil
	}

	resolvedPath, err := resolveBinaryPath(siblingName)
	if err != nil {
		return "", fmt.Errorf("resolve %s binary: %w", siblingName, err)
	}

	return resolvedPath, nil
}

func (service *WhatsAppService) fetchImage(ctx context.Context, imageURL string) ([]byte, error) {
	if service.dependencies.HTTPClient == nil {
		return nil, fmt.Errorf("http client not available")
	}

	parsedURL, err := url.ParseRequestURI(imageURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid image url")
	}

	payload, err := service.dependencies.HTTPClient.Get(ctx, imageURL)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty image payload")
	}

	return payload, nil
}

func (service *WhatsAppService) callRemoveBG(ctx context.Context, imageBytes []byte, color string) ([]byte, error) {
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	fileWriter, err := writer.CreateFormFile("image_file", "input.jpg")
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	_, err = fileWriter.Write(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("write form file: %w", err)
	}

	if err := writer.WriteField("bg_color", color); err != nil {
		return nil, fmt.Errorf("write bg_color field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(service.dependencies.Config.Api.RemoveBgUrl, "/")+"/v1.0/removebg",
		bodyBuffer,
	)
	if err != nil {
		return nil, fmt.Errorf("build removebg request: %w", err)
	}
	request.Header.Set("X-Api-Key", service.dependencies.Config.Media.RemoveBgApiKey)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	httpClient := &http.Client{Timeout: 60 * time.Second}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("execute removebg request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read removebg response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("removebg failed with status %d", response.StatusCode)
	}

	return responseBody, nil
}

func (service *WhatsAppService) getFirstReachable(ctx context.Context, urls []string) ([]byte, error) {
	if service.dependencies.HTTPClient == nil {
		return nil, fmt.Errorf("http client not available")
	}

	var lastErr error
	for _, endpoint := range deduplicateStrings(urls) {
		payload, err := service.dependencies.HTTPClient.Get(ctx, endpoint)
		if err == nil {
			return payload, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("no candidate endpoint")
}

func parseCommand(input string, commandMap map[string]commandHandler) (string, string, bool) {
	content := helper.NormalizeMessage(input)
	if content == "" {
		return "", "", false
	}

	explicit := strings.HasPrefix(content, "!") || strings.HasPrefix(content, "/")
	if !explicit {
		return "", "", false
	}
	content = strings.TrimSpace(content[1:])
	if content == "" {
		return "", "", explicit
	}

	parts := strings.SplitN(content, " ", 2)
	commandName := strings.ToLower(strings.TrimSpace(parts[0]))
	if commandName == "" {
		return "", "", explicit
	}
	if at := strings.Index(commandName, "@"); at >= 0 {
		commandName = commandName[:at]
	}

	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	commandName = "!" + commandName

	if _, exists := commandMap[commandName]; exists {
		return commandName, args, true
	}
	return commandName, args, true
}

func extractResultList(value any) []map[string]any {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	resultRaw, exists := root["result"]
	if !exists {
		return nil
	}

	resultItems, ok := resultRaw.([]any)
	if !ok {
		return nil
	}

	mapped := make([]map[string]any, 0, len(resultItems))
	for _, item := range resultItems {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		mapped = append(mapped, itemMap)
	}

	return mapped
}

func extractCity(value any) (string, string) {
	mapped, ok := value.(map[string]any)
	if !ok {
		return "", ""
	}

	data, ok := mapped["data"].([]any)
	if !ok || len(data) == 0 {
		return "", ""
	}

	firstItem, ok := data[0].(map[string]any)
	if !ok {
		return "", ""
	}

	cityID := helper.FirstNonEmpty(helper.StringValue(firstItem["id"]), helper.StringValue(firstItem["value"]))
	cityName := helper.FirstNonEmpty(helper.StringValue(firstItem["lokasi"]), helper.StringValue(firstItem["kota"]), helper.StringValue(firstItem["nama"]))

	return cityID, cityName
}

func extractScheduleMap(value any) map[string]any {
	mapped, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	data, ok := mapped["data"].(map[string]any)
	if !ok {
		return nil
	}

	jadwal, ok := data["jadwal"].(map[string]any)
	if !ok {
		return nil
	}

	return jadwal
}

func parseColorAndURL(args string) (string, string, bool) {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) < 2 {
		return "", "", false
	}

	color := strings.ToLower(strings.TrimSpace(parts[0]))
	imageURL := strings.TrimSpace(parts[1])
	if color == "" || imageURL == "" {
		return "", "", false
	}

	return color, imageURL, true
}

func compactJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}

	text := strings.TrimSpace(string(payload))
	if len(text) > 3000 {
		return text[:3000] + "..."
	}

	return text
}

func looksLikeHTML(payload []byte) bool {
	text := strings.ToLower(strings.TrimSpace(string(payload)))
	if text == "" {
		return false
	}

	return strings.HasPrefix(text, "<!doctype html") ||
		strings.HasPrefix(text, "<html") ||
		strings.Contains(text, "<head") ||
		strings.Contains(text, "<body")
}

func deduplicateStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}

func (service *WhatsAppService) resolveEventOrQuotedMedia(event *whatsapp.IncomingMessageEvent) ([]byte, string, string) {
	if len(event.MediaData) > 0 {
		return event.MediaData, strings.TrimSpace(event.MediaType), strings.TrimSpace(event.MediaMime)
	}

	if cached, ok := service.getCachedMedia(event.QuotedMessageId); ok {
		return cached.MediaData, strings.TrimSpace(cached.MediaType), strings.TrimSpace(cached.MediaMime)
	}

	return nil, "", ""
}

func extractDoaList(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				result = append(result, mapped)
			}
		}
		return result
	case map[string]any:
		if dataRaw, exists := typed["data"]; exists {
			if dataList, ok := dataRaw.([]any); ok {
				result := make([]map[string]any, 0, len(dataList))
				for _, item := range dataList {
					if mapped, ok := item.(map[string]any); ok {
						result = append(result, mapped)
					}
				}
				return result
			}
		}
		if mapped := helper.PickFirstMap(typed); mapped != nil {
			return []map[string]any{mapped}
		}
	}

	return nil
}

func extractAsmaulHusnaList(value any) []map[string]any {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	dataRaw, exists := root["data"]
	if !exists {
		if mapped := helper.PickFirstMap(root); mapped != nil {
			return []map[string]any{mapped}
		}
		return nil
	}

	dataList, ok := dataRaw.([]any)
	if !ok {
		if mapped, ok := dataRaw.(map[string]any); ok {
			return []map[string]any{mapped}
		}
		return nil
	}

	result := make([]map[string]any, 0, len(dataList))
	for _, item := range dataList {
		if mapped, ok := item.(map[string]any); ok {
			result = append(result, mapped)
		}
	}

	return result
}

func extractStringList(root map[string]any, key string) []string {
	raw, exists := root[key]
	if !exists {
		return nil
	}

	rawList, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(rawList))
	for _, item := range rawList {
		text := helper.StringValue(item)
		if text == "" {
			continue
		}
		result = append(result, text)
	}

	return result
}

func (service *WhatsAppService) sendWaitMessage(ctx context.Context, chatID string) {
	if strings.TrimSpace(chatID) == "" {
		return
	}
	_, _ = service.dependencies.WhatsAppClient.SendText(ctx, &whatsapp.SendTextRequest{
		ChatId: chatID,
		Text:   waitMessage,
	})
}

func (service *WhatsAppService) cacheIncomingMedia(event *whatsapp.IncomingMessageEvent) {
	if strings.TrimSpace(event.MessageId) == "" {
		return
	}
	if len(event.MediaData) == 0 {
		return
	}
	if event.MediaType != "image" && event.MediaType != "video" {
		return
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	service.mediaCache[event.MessageId] = cachedMedia{
		MediaType: event.MediaType,
		MediaMime: event.MediaMime,
		MediaData: append([]byte(nil), event.MediaData...),
		CreatedAt: time.Now(),
	}
	service.pruneMediaCacheLocked(200, 30*time.Minute)
}

func (service *WhatsAppService) getCachedMedia(messageID string) (cachedMedia, bool) {
	if strings.TrimSpace(messageID) == "" {
		return cachedMedia{}, false
	}

	service.mu.RLock()
	defer service.mu.RUnlock()

	value, exists := service.mediaCache[messageID]
	if !exists {
		return cachedMedia{}, false
	}
	if time.Since(value.CreatedAt) > 30*time.Minute {
		return cachedMedia{}, false
	}

	return cachedMedia{
		MediaType: value.MediaType,
		MediaMime: value.MediaMime,
		MediaData: append([]byte(nil), value.MediaData...),
		CreatedAt: value.CreatedAt,
	}, true
}

func (service *WhatsAppService) pruneMediaCacheLocked(maxItems int, maxAge time.Duration) {
	if len(service.mediaCache) <= maxItems {
		now := time.Now()
		for key, value := range service.mediaCache {
			if now.Sub(value.CreatedAt) > maxAge {
				delete(service.mediaCache, key)
			}
		}
		return
	}

	now := time.Now()
	for key, value := range service.mediaCache {
		if now.Sub(value.CreatedAt) > maxAge {
			delete(service.mediaCache, key)
		}
	}

	if len(service.mediaCache) <= maxItems {
		return
	}

	type pair struct {
		Key string
		At  time.Time
	}
	pairs := make([]pair, 0, len(service.mediaCache))
	for key, value := range service.mediaCache {
		pairs = append(pairs, pair{Key: key, At: value.CreatedAt})
	}
	for len(pairs) > maxItems {
		oldestIndex := 0
		for i := 1; i < len(pairs); i++ {
			if pairs[i].At.Before(pairs[oldestIndex].At) {
				oldestIndex = i
			}
		}
		delete(service.mediaCache, pairs[oldestIndex].Key)
		pairs = append(pairs[:oldestIndex], pairs[oldestIndex+1:]...)
	}
}

func detectStickerMediaKind(mediaType string, mediaMime string, mediaBytes []byte) string {
	normalizedType := strings.ToLower(strings.TrimSpace(mediaType))
	normalizedMime := strings.ToLower(strings.TrimSpace(mediaMime))
	if normalizedMime == "" && len(mediaBytes) > 0 {
		normalizedMime = strings.ToLower(http.DetectContentType(mediaBytes))
	}

	if normalizedType == "video" {
		return "animated"
	}
	if strings.HasPrefix(normalizedMime, "video/") {
		return "animated"
	}
	if strings.Contains(normalizedMime, "gif") {
		return "animated"
	}
	if strings.HasPrefix(normalizedMime, "image/") {
		return "static"
	}

	return "unknown"
}

func buildStaticSticker(mediaBytes []byte) ([]byte, error) {
	if len(mediaBytes) == 0 {
		return nil, fmt.Errorf("image bytes is empty")
	}

	sourceImage, _, err := image.Decode(bytes.NewReader(mediaBytes))
	if err != nil {
		return nil, fmt.Errorf("decode source image: %w", err)
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	fillTransparent(canvas)
	drawImageFitCenter(canvas, sourceImage)

	var output bytes.Buffer
	if err := webp.Encode(&output, canvas, &webp.Options{Lossless: true, Quality: 90}); err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}

	return output.Bytes(), nil
}

func buildBratSticker(text string) ([]byte, error) {
	imageBytes, err := renderBratImage(text)
	if err != nil {
		return nil, err
	}

	return buildStaticSticker(imageBytes)
}

func renderBratImage(text string) ([]byte, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	if normalized == "" {
		return nil, fmt.Errorf("empty brat text")
	}

	const (
		canvasSize  = 512
		maxTextArea = canvasSize - 48
		maxLines    = 6
		maxRunes    = 280
	)

	if utf8.RuneCountInString(normalized) > maxRunes {
		runes := []rune(normalized)
		normalized = string(runes[:maxRunes])
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	ttfBytes, err := loadBratFontTTF()
	if err != nil {
		return nil, err
	}

	ttf, err := opentype.Parse(ttfBytes)
	if err != nil {
		return nil, fmt.Errorf("parse brat font: %w", err)
	}

	bestSize := 130.0
	bestGap := 8
	var face font.Face
	var lines []string
	var lineHeight int
	var ascent int

	for size := 130.0; size >= 50; size -= 4 {
		candidateFace, faceErr := opentype.NewFace(ttf, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if faceErr != nil {
			return nil, fmt.Errorf("create brat font face: %w", faceErr)
		}

		candidateLines := wrapBratLines(normalized, candidateFace, maxTextArea)
		if len(candidateLines) == 0 {
			candidateFace.Close()
			continue
		}
		if len(candidateLines) > maxLines {
			candidateFace.Close()
			continue
		}

		metrics := candidateFace.Metrics()
		candidateLineHeight := metrics.Height.Ceil()
		candidateAscent := metrics.Ascent.Ceil()
		gap := int(size * 0.12)
		if gap < 6 {
			gap = 6
		}
		totalHeight := len(candidateLines)*candidateLineHeight + (len(candidateLines)-1)*gap
		if totalHeight > canvasSize-44 {
			candidateFace.Close()
			continue
		}

		face = candidateFace
		lines = candidateLines
		lineHeight = candidateLineHeight
		ascent = candidateAscent
		bestSize = size
		bestGap = gap
		break
	}

	if face == nil {
		fallbackFace, faceErr := opentype.NewFace(ttf, &opentype.FaceOptions{
			Size:    bestSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if faceErr != nil {
			return nil, fmt.Errorf("create fallback brat font face: %w", faceErr)
		}
		face = fallbackFace
		lines = wrapBratLines(normalized, face, maxTextArea)
		if len(lines) > maxLines {
			lines = lines[:maxLines]
		}
		if len(lines) == 0 {
			face.Close()
			return nil, fmt.Errorf("empty brat lines")
		}
		last := strings.TrimSpace(lines[len(lines)-1])
		if utf8.RuneCountInString(last) > 2 {
			runes := []rune(last)
			lines[len(lines)-1] = string(runes[:len(runes)-1]) + "…"
		}
		metrics := face.Metrics()
		lineHeight = metrics.Height.Ceil()
		ascent = metrics.Ascent.Ceil()
	}
	defer face.Close()

	textColor := image.NewUniform(color.Black)
	const (
		leftPadding = 16
		topPadding  = 16
	)
	y := topPadding + ascent
	drawer := &font.Drawer{Dst: canvas, Src: textColor, Face: face}

	for _, line := range lines {
		drawer.Dot.X = fixed.I(leftPadding)
		drawer.Dot.Y = fixed.I(y)
		drawer.DrawString(line)
		y += lineHeight + bestGap
	}

	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, fmt.Errorf("encode brat image: %w", err)
	}

	return output.Bytes(), nil
}

func wrapBratLines(text string, face font.Face, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	drawer := &font.Drawer{Face: face}
	lines := make([]string, 0, len(words))
	current := ""

	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if drawer.MeasureString(candidate).Ceil() <= maxWidth {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}

		if drawer.MeasureString(word).Ceil() <= maxWidth {
			current = word
			continue
		}

		parts := splitWordByWidth(word, drawer, maxWidth)
		if len(parts) == 0 {
			continue
		}
		lines = append(lines, parts[:len(parts)-1]...)
		current = parts[len(parts)-1]
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}

func splitWordByWidth(word string, drawer *font.Drawer, maxWidth int) []string {
	runes := []rune(word)
	if len(runes) == 0 {
		return nil
	}

	parts := make([]string, 0, 2)
	start := 0
	for start < len(runes) {
		end := start + 1
		for end <= len(runes) && drawer.MeasureString(string(runes[start:end])).Ceil() <= maxWidth {
			end++
		}
		if end == start+1 {
			end = start + 2
			if end > len(runes) {
				end = len(runes)
			}
		} else {
			end--
		}
		parts = append(parts, string(runes[start:end]))
		start = end
	}

	return parts
}

func loadBratFontTTF() ([]byte, error) {
	candidates := []string{}
	if custom := strings.TrimSpace(os.Getenv("BRAT_FONT_PATH")); custom != "" {
		candidates = append(candidates, custom)
		if !filepath.IsAbs(custom) {
			candidates = append(candidates, filepath.Join("/app", custom))
		}
	}
	candidates = append(candidates,
		"/app/fonts/arial-narrow/arialnarrow.ttf",
		"/app/fonts/arial-narrow/ArialNarrow.ttf",
		"/usr/share/fonts/truetype/msttcorefonts/ArialNarrow.ttf",
		"/usr/share/fonts/truetype/msttcorefonts/Arial Narrow.ttf",
		"/Library/Fonts/Arial Narrow.ttf",
		"/System/Library/Fonts/Supplemental/Arial Narrow.ttf",
		"/usr/share/fonts/liberation/LiberationSansNarrow-Regular.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSansNarrow-Regular.ttf",
	)

	for _, path := range candidates {
		fontBytes, err := os.ReadFile(path)
		if err == nil && len(fontBytes) > 0 {
			return fontBytes, nil
		}
	}

	return nil, fmt.Errorf("brat font not found")
}

func fillTransparent(target *image.NRGBA) {
	bounds := target.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			target.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
		}
	}
}

func drawImageFitCenter(target *image.NRGBA, source image.Image) {
	srcBounds := source.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	if srcWidth <= 0 || srcHeight <= 0 {
		return
	}

	scaleX := float64(512) / float64(srcWidth)
	scaleY := float64(512) / float64(srcHeight)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}
	if scale <= 0 {
		return
	}

	scaledWidth := int(float64(srcWidth) * scale)
	scaledHeight := int(float64(srcHeight) * scale)
	if scaledWidth < 1 {
		scaledWidth = 1
	}
	if scaledHeight < 1 {
		scaledHeight = 1
	}

	offsetX := (512 - scaledWidth) / 2
	offsetY := (512 - scaledHeight) / 2

	for y := 0; y < scaledHeight; y++ {
		srcY := srcBounds.Min.Y + int(float64(y)*float64(srcHeight)/float64(scaledHeight))
		for x := 0; x < scaledWidth; x++ {
			srcX := srcBounds.Min.X + int(float64(x)*float64(srcWidth)/float64(scaledWidth))
			target.Set(offsetX+x, offsetY+y, source.At(srcX, srcY))
		}
	}
}

func runCommand(ctx context.Context, binaryPath string, args ...string) error {
	command := exec.CommandContext(ctx, binaryPath, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return fmt.Errorf("%s command failed: %w", binaryPath, err)
		}
		return fmt.Errorf("%s command failed: %w: %s", binaryPath, err, trimmed)
	}

	return nil
}

func normalizeVideoForWhatsApp(videoBytes []byte, mimeType string, force bool) ([]byte, string, error) {
	normalizedMime := strings.ToLower(strings.TrimSpace(mimeType))
	if len(videoBytes) == 0 {
		return nil, "", fmt.Errorf("video bytes is empty")
	}
	if !force && (normalizedMime == "video/mp4" || normalizedMime == "") {
		return videoBytes, "video/mp4", nil
	}

	ffmpegBinary, err := resolveBinaryPath("ffmpeg")
	if err != nil {
		return nil, "", err
	}

	tempDir, err := os.MkdirTemp("", "ryuko-video-normalize-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input-video.bin")
	outputPath := filepath.Join(tempDir, "output-video.mp4")

	if err := os.WriteFile(inputPath, videoBytes, 0o600); err != nil {
		return nil, "", fmt.Errorf("write temp input file: %w", err)
	}

	ffmpegArgs := []string{
		"-y",
		"-i", inputPath,
		"-c:v", "libx264",
		"-profile:v", "main",
		"-level", "4.0",
		"-preset", "veryfast",
		"-c:a", "aac",
		"-b:a", "128k",
		"-max_muxing_queue_size", "1024",
		"-movflags", "+faststart",
		"-pix_fmt", "yuv420p",
		outputPath,
	}
	if err := runCommand(context.Background(), ffmpegBinary, ffmpegArgs...); err != nil {
		return nil, "", fmt.Errorf("transcode video for whatsapp: %w", err)
	}

	resultBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, "", fmt.Errorf("read normalized video: %w", err)
	}
	if len(resultBytes) == 0 {
		return nil, "", fmt.Errorf("normalized video is empty")
	}

	return resultBytes, "video/mp4", nil
}

func logDownloaderFailure(platform string, rawURL string, err error) {
	message := ""
	if err != nil {
		message = strings.Join(strings.Fields(strings.TrimSpace(err.Error())), " ")
	}
	if len(message) > 2000 {
		message = message[:2000] + "..."
	}

	log.Printf(
		"downloader_failure platform=%s url=%s error=%s",
		strings.TrimSpace(platform),
		strings.TrimSpace(rawURL),
		message,
	)
}

func resolveBinaryPath(name string) (string, error) {
	switch name {
	case "ffmpeg":
		if custom := strings.TrimSpace(os.Getenv("FFMPEG_PATH")); custom != "" {
			if _, err := os.Stat(custom); err == nil {
				return custom, nil
			}
		}
	case "yt-dlp":
		if custom := strings.TrimSpace(os.Getenv("YTDLP_PATH")); custom != "" {
			if _, err := os.Stat(custom); err == nil {
				return custom, nil
			}
		}
	}

	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	knownPaths := map[string][]string{
		"ffmpeg": {
			"/opt/homebrew/bin/ffmpeg",
			"/usr/local/bin/ffmpeg",
			"/usr/bin/ffmpeg",
		},
		"yt-dlp": {
			"/opt/homebrew/bin/yt-dlp",
			"/usr/local/bin/yt-dlp",
			"/usr/bin/yt-dlp",
		},
	}

	for _, candidate := range knownPaths[name] {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("binary %s tidak ditemukan. Set env path: %s", name, binaryPathEnvHint(name))
}

func binaryPathEnvHint(name string) string {
	if name == "ffmpeg" {
		return "FFMPEG_PATH"
	}
	if name == "yt-dlp" {
		return "YTDLP_PATH"
	}
	return "PATH"
}
