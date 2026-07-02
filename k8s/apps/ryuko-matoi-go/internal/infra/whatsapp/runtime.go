package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	_ "modernc.org/sqlite"

	"github.com/chai2010/webp"
)

type Runtime struct {
	client         *whatsmeow.Client
	store          *sqlstore.Container
	messageHandler MessageHandler
	logger         waLog.Logger
}

type RuntimeDependencies struct {
	Config *Config
	Logger waLog.Logger
}

func NewRuntime(ctx context.Context, dependencies RuntimeDependencies) (*Runtime, error) {
	logger := dependencies.Logger
	if logger == nil {
		logger = waLog.Stdout("WhatsMeow", "INFO", true)
	}

	storeContainer, err := sqlstore.New(ctx, dependencies.Config.DatabaseDialect, dependencies.Config.DatabaseDsn, logger)
	if err != nil {
		return nil, fmt.Errorf("create whatsapp sql store: %w", err)
	}

	deviceStore, err := storeContainer.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get whatsapp device store: %w", err)
	}

	client := whatsmeow.NewClient(deviceStore, logger)

	runtime := &Runtime{
		client: client,
		store:  storeContainer,
		logger: logger,
	}

	client.AddEventHandler(runtime.handleEvent)

	return runtime, nil
}

func (runtime *Runtime) IsAuthenticated() bool {
	if runtime.client == nil || runtime.client.Store == nil {
		return false
	}

	return runtime.client.Store.ID != nil
}

func (runtime *Runtime) QRChannel(ctx context.Context) (<-chan *QREvent, error) {
	channel, err := runtime.client.GetQRChannel(ctx)
	if err != nil {
		return nil, fmt.Errorf("get qr channel: %w", err)
	}

	result := make(chan *QREvent)

	go func() {
		defer close(result)
		for event := range channel {
			errStr := ""
			if event.Error != nil {
				errStr = event.Error.Error()
			}
			result <- &QREvent{
				Event: event.Event,
				Code:  event.Code,
				Error: errStr,
			}
		}
	}()

	return result, nil
}

func (runtime *Runtime) Connect(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := runtime.client.Connect(); err != nil {
		return fmt.Errorf("connect whatsapp client: %w", err)
	}

	return nil
}

func (runtime *Runtime) Disconnect(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if runtime.client != nil {
		runtime.client.Disconnect()
	}

	if runtime.store != nil {
		if err := runtime.store.Close(); err != nil {
			return fmt.Errorf("close whatsapp store: %w", err)
		}
	}

	return nil
}

func (runtime *Runtime) SendText(ctx context.Context, request *SendTextRequest) (*SendTextResponse, error) {
	jid, err := types.ParseJID(request.ChatId)
	if err != nil {
		return nil, fmt.Errorf("parse chat jid: %w", err)
	}

	message := &waProto.Message{
		Conversation: proto.String(request.Text),
	}

	response, err := runtime.client.SendMessage(ctx, jid, message)
	if err != nil {
		return nil, fmt.Errorf("send whatsapp message: %w", err)
	}

	return &SendTextResponse{MessageId: response.ID}, nil
}

func (runtime *Runtime) SendImage(ctx context.Context, request *SendImageRequest) (*SendImageResponse, error) {
	jid, err := types.ParseJID(request.ChatId)
	if err != nil {
		return nil, fmt.Errorf("parse chat jid: %w", err)
	}
	if len(request.ImageBytes) == 0 {
		return nil, fmt.Errorf("image bytes is empty")
	}

	upload, err := runtime.client.Upload(ctx, request.ImageBytes, whatsmeow.MediaImage)
	if err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}

	mimeType := http.DetectContentType(request.ImageBytes)
	imageMessage := &waProto.ImageMessage{
		Caption:       proto.String(request.Caption),
		Mimetype:      proto.String(mimeType),
		URL:           &upload.URL,
		DirectPath:    &upload.DirectPath,
		MediaKey:      upload.MediaKey,
		FileEncSHA256: upload.FileEncSHA256,
		FileSHA256:    upload.FileSHA256,
		FileLength:    &upload.FileLength,
	}

	response, err := runtime.client.SendMessage(ctx, jid, &waProto.Message{
		ImageMessage: imageMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("send image message: %w", err)
	}

	return &SendImageResponse{MessageId: response.ID}, nil
}

func AddExifToWebP(webpData []byte, packName, author string) ([]byte, error) {
	metadata := map[string]any{
		"sticker-pack-id":        "com.ryuko.matoi.stickers",
		"sticker-pack-name":      packName,
		"sticker-pack-publisher": author,
		"android-app-store-link": "https://play.google.com/store/apps/details?id=com.marsvard.stickermakerforwhatsapp",
		"ios-app-store-link":     "https://itunes.apple.com/app/sticker-maker-studio/id1443326857",
		"emojis":                 []string{""},
	}

	jsonBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal sticker exif metadata: %w", err)
	}

	// WhatsApp sticker pack metadata uses a custom EXIF payload with the AW tag.
	exifHeader := []byte{
		0x49, 0x49, 0x2A, 0x00,
		0x08, 0x00, 0x00, 0x00,
		0x01, 0x00,
		0x41, 0x57,
		0x07, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x16, 0x00, 0x00, 0x00,
	}
	writeUint32LE(exifHeader[14:18], uint32(len(jsonBytes)))

	exifPayload := append(exifHeader, jsonBytes...)
	result, err := webp.SetMetadata(webpData, exifPayload, "EXIF")
	if err != nil {
		return nil, fmt.Errorf("set webp exif metadata: %w", err)
	}
	return result, nil
}

func writeUint32LE(target []byte, value uint32) {
	if len(target) < 4 {
		return
	}
	target[0] = byte(value)
	target[1] = byte(value >> 8)
	target[2] = byte(value >> 16)
	target[3] = byte(value >> 24)
}

func isAnimatedWebP(webpData []byte) bool {
	if len(webpData) == 0 {
		return false
	}
	// Animated WebP umumnya memiliki chunk ANIM/ANMF.
	return bytes.Contains(webpData, []byte("ANIM")) || bytes.Contains(webpData, []byte("ANMF"))
}

func (runtime *Runtime) SendSticker(ctx context.Context, request *SendStickerRequest) (*SendStickerResponse, error) {
	jid, err := types.ParseJID(request.ChatId)
	if err != nil {
		return nil, fmt.Errorf("parse chat jid: %w", err)
	}
	if len(request.StickerWebp) == 0 {
		return nil, fmt.Errorf("sticker bytes is empty")
	}

	webpWithMeta, err := AddExifToWebP(request.StickerWebp, "Ryuko Matoi", "github.com/haikelz")
	if err != nil {
		return nil, fmt.Errorf("add exif to webp: %w", err)
	}
	if len(webpWithMeta) == 0 {
		return nil, fmt.Errorf("sticker bytes is empty after exif")
	}

	stickerWidth := request.Width
	stickerHeight := request.Height
	if stickerWidth == 0 {
		stickerWidth = 512
	}
	if stickerHeight == 0 {
		stickerHeight = 512
	}

	upload, err := runtime.client.Upload(ctx, webpWithMeta, whatsmeow.MediaImage)
	if err != nil {
		return nil, fmt.Errorf("upload sticker: %w", err)
	}

	stickerMessage := &waProto.StickerMessage{
		Mimetype:           proto.String("image/webp"),
		URL:                proto.String(upload.URL),
		DirectPath:         &upload.DirectPath,
		MediaKey:           upload.MediaKey,
		FileEncSHA256:      upload.FileEncSHA256,
		FileSHA256:         upload.FileSHA256,
		FileLength:         &upload.FileLength,
		Width:              proto.Uint32(stickerWidth),
		Height:             proto.Uint32(stickerHeight),
		IsAnimated:         proto.Bool(isAnimatedWebP(webpWithMeta)),
		AccessibilityLabel: proto.String("Ryuko Matoi Sticker"),
	}

	response, err := runtime.client.SendMessage(ctx, jid, &waProto.Message{
		StickerMessage: stickerMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("send sticker message: %w", err)
	}

	return &SendStickerResponse{MessageId: response.ID}, nil
}

func (runtime *Runtime) SendVideo(ctx context.Context, request *SendVideoRequest) (*SendVideoResponse, error) {
	jid, err := types.ParseJID(request.ChatId)
	if err != nil {
		return nil, fmt.Errorf("parse chat jid: %w", err)
	}
	if len(request.VideoBytes) == 0 {
		return nil, fmt.Errorf("video bytes is empty")
	}

	upload, err := runtime.client.Upload(ctx, request.VideoBytes, whatsmeow.MediaVideo)
	if err != nil {
		return nil, fmt.Errorf("upload video: %w", err)
	}

	mimeType := strings.TrimSpace(request.MimeType)
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	seconds, width, height, thumbnail := probeVideoMetadata(ctx, request.VideoBytes)

	videoMessage := &waProto.VideoMessage{
		Caption:       proto.String(request.Caption),
		Mimetype:      proto.String(mimeType),
		URL:           &upload.URL,
		DirectPath:    &upload.DirectPath,
		MediaKey:      upload.MediaKey,
		FileEncSHA256: upload.FileEncSHA256,
		FileSHA256:    upload.FileSHA256,
		FileLength:    &upload.FileLength,
		GifPlayback:   proto.Bool(request.GifPlayback),
	}
	if seconds > 0 {
		videoMessage.Seconds = proto.Uint32(seconds)
	}
	if width > 0 {
		videoMessage.Width = proto.Uint32(width)
	}
	if height > 0 {
		videoMessage.Height = proto.Uint32(height)
	}
	if len(thumbnail) > 0 {
		videoMessage.JPEGThumbnail = thumbnail
	}
	if request.Ptt {
		videoMessage.GifPlayback = proto.Bool(false)
	}

	response, err := runtime.client.SendMessage(ctx, jid, &waProto.Message{
		VideoMessage: videoMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("send video message: %w", err)
	}

	return &SendVideoResponse{MessageId: response.ID}, nil
}

func probeVideoMetadata(ctx context.Context, videoBytes []byte) (uint32, uint32, uint32, []byte) {
	if len(videoBytes) == 0 {
		return 0, 0, 0, nil
	}

	ffprobePath, ffprobeErr := exec.LookPath("ffprobe")
	ffmpegPath, ffmpegErr := exec.LookPath("ffmpeg")
	if ffprobeErr != nil && ffmpegErr != nil {
		return 0, 0, 0, nil
	}

	tempDir, err := os.MkdirTemp("", "ryuko-video-meta-*")
	if err != nil {
		return 0, 0, 0, nil
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, "input.mp4")
	if err := os.WriteFile(videoPath, videoBytes, 0o600); err != nil {
		return 0, 0, 0, nil
	}

	var seconds, width, height uint32
	if ffprobeErr == nil {
		probeCmd := exec.CommandContext(
			ctx,
			ffprobePath,
			"-v", "error",
			"-select_streams", "v:0",
			"-show_entries", "stream=width,height,duration",
			"-of", "csv=p=0",
			videoPath,
		)
		if output, err := probeCmd.Output(); err == nil {
			parts := strings.Split(strings.TrimSpace(string(output)), ",")
			if len(parts) >= 2 {
				if value, parseErr := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32); parseErr == nil {
					width = uint32(value)
				}
				if value, parseErr := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 32); parseErr == nil {
					height = uint32(value)
				}
			}
			if len(parts) >= 3 {
				if value, parseErr := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); parseErr == nil && value > 0 {
					seconds = uint32(value + 0.5)
				}
			}
		}

		if seconds == 0 {
			durationCmd := exec.CommandContext(
				ctx,
				ffprobePath,
				"-v", "error",
				"-show_entries", "format=duration",
				"-of", "default=noprint_wrappers=1:nokey=1",
				videoPath,
			)
			if output, err := durationCmd.Output(); err == nil {
				if value, parseErr := strconv.ParseFloat(strings.TrimSpace(string(output)), 64); parseErr == nil && value > 0 {
					seconds = uint32(value + 0.5)
				}
			}
		}
	}

	if ffmpegErr != nil {
		return seconds, width, height, nil
	}

	thumbPath := filepath.Join(tempDir, "thumb.jpg")
	thumbCmd := exec.CommandContext(
		ctx,
		ffmpegPath,
		"-y",
		"-i", videoPath,
		"-ss", "00:00:00.500",
		"-vframes", "1",
		"-f", "image2",
		thumbPath,
	)
	if output, err := thumbCmd.CombinedOutput(); err != nil {
		if runtimeErr := strings.TrimSpace(string(output)); runtimeErr != "" && runtimeErr != "At least one output file must be specified" {
			return seconds, width, height, nil
		}
	}

	thumbnail, err := os.ReadFile(thumbPath)
	if err != nil || len(thumbnail) == 0 {
		return seconds, width, height, nil
	}

	return seconds, width, height, thumbnail
}

func (runtime *Runtime) RegisterMessageHandler(handler MessageHandler) {
	runtime.messageHandler = handler
}

func (runtime *Runtime) handleEvent(event any) {
	if runtime.messageHandler == nil {
		return
	}

	messageEvent, ok := event.(*events.Message)
	if !ok {
		return
	}
	messageEvent = messageEvent.UnwrapRaw()
	if messageEvent == nil || messageEvent.Message == nil {
		return
	}
	message := messageEvent.Message

	body := message.GetConversation()
	if body == "" && message.GetExtendedTextMessage() != nil {
		body = message.GetExtendedTextMessage().GetText()
	}

	mediaType := ""
	mediaMime := ""
	var mediaData []byte

	if imageMessage := message.GetImageMessage(); imageMessage != nil {
		if body == "" {
			body = strings.TrimSpace(imageMessage.GetCaption())
		}
		mediaType = "image"
		mediaMime = imageMessage.GetMimetype()
		downloaded, err := runtime.client.Download(context.Background(), imageMessage)
		if err == nil {
			mediaData = downloaded
		}
	} else if videoMessage := message.GetVideoMessage(); videoMessage != nil {
		if body == "" {
			body = strings.TrimSpace(videoMessage.GetCaption())
		}
		mediaType = "video"
		mediaMime = videoMessage.GetMimetype()
		downloaded, err := runtime.client.Download(context.Background(), videoMessage)
		if err == nil {
			mediaData = downloaded
		}
	} else if documentMessage := message.GetDocumentMessage(); documentMessage != nil {
		if body == "" {
			body = strings.TrimSpace(documentMessage.GetCaption())
		}
		mediaMime = strings.TrimSpace(documentMessage.GetMimetype())
		if strings.HasPrefix(mediaMime, "image/") {
			mediaType = "image"
		} else if strings.HasPrefix(mediaMime, "video/") || strings.Contains(mediaMime, "gif") {
			mediaType = "video"
		}
		if mediaType != "" {
			downloaded, err := runtime.client.Download(context.Background(), documentMessage)
			if err == nil {
				mediaData = downloaded
			}
		}
	}

	if len(mediaData) == 0 {
		if extended := message.GetExtendedTextMessage(); extended != nil {
			if contextInfo := extended.GetContextInfo(); contextInfo != nil {
				if quoted := contextInfo.GetQuotedMessage(); quoted != nil {
					if quotedImage := quoted.GetImageMessage(); quotedImage != nil {
						mediaType = "image"
						mediaMime = quotedImage.GetMimetype()
						downloaded, err := runtime.client.Download(context.Background(), quotedImage)
						if err == nil {
							mediaData = downloaded
						}
					} else if quotedVideo := quoted.GetVideoMessage(); quotedVideo != nil {
						mediaType = "video"
						mediaMime = quotedVideo.GetMimetype()
						downloaded, err := runtime.client.Download(context.Background(), quotedVideo)
						if err == nil {
							mediaData = downloaded
						}
					}
				}
			}
		}
	}

	incomingEvent := &IncomingMessageEvent{
		MessageId:       messageEvent.Info.ID,
		ChatId:          messageEvent.Info.Chat.String(),
		SenderJid:       messageEvent.Info.Sender.String(),
		FromMe:          messageEvent.Info.IsFromMe,
		Body:            body,
		QuotedMessageId: extractQuotedMessageID(message),
		MediaType:       mediaType,
		MediaMime:       mediaMime,
		MediaData:       mediaData,
		Timestamp:       timestamppb.New(messageEvent.Info.Timestamp),
	}

	if err := runtime.messageHandler(context.Background(), incomingEvent); err != nil {
		if runtime.logger != nil {
			runtime.logger.Errorf("whatsapp message handler error: %v", err)
		}
	}
}

func extractQuotedMessageID(message *waProto.Message) string {
	if message == nil {
		return ""
	}

	if ext := message.GetExtendedTextMessage(); ext != nil {
		if ctx := ext.GetContextInfo(); ctx != nil {
			if stanzaID := strings.TrimSpace(ctx.GetStanzaID()); stanzaID != "" {
				return stanzaID
			}
		}
	}
	if img := message.GetImageMessage(); img != nil {
		if ctx := img.GetContextInfo(); ctx != nil {
			if stanzaID := strings.TrimSpace(ctx.GetStanzaID()); stanzaID != "" {
				return stanzaID
			}
		}
	}
	if vid := message.GetVideoMessage(); vid != nil {
		if ctx := vid.GetContextInfo(); ctx != nil {
			if stanzaID := strings.TrimSpace(ctx.GetStanzaID()); stanzaID != "" {
				return stanzaID
			}
		}
	}
	if doc := message.GetDocumentMessage(); doc != nil {
		if ctx := doc.GetContextInfo(); ctx != nil {
			if stanzaID := strings.TrimSpace(ctx.GetStanzaID()); stanzaID != "" {
				return stanzaID
			}
		}
	}

	return ""
}
