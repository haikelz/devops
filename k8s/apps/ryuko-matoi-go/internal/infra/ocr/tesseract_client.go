package ocr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type TesseractClient struct {
	binaryPath string
	language   string
}

func NewTesseractClient(binaryPath string, language string) *TesseractClient {
	trimmedBinary := strings.TrimSpace(binaryPath)
	if trimmedBinary == "" {
		trimmedBinary = "tesseract"
	}

	trimmedLanguage := strings.TrimSpace(language)
	if trimmedLanguage == "" {
		trimmedLanguage = "eng"
	}

	return &TesseractClient{
		binaryPath: trimmedBinary,
		language:   trimmedLanguage,
	}
}

func (client *TesseractClient) ExtractText(ctx context.Context, content []byte) (string, error) {
	if len(content) == 0 {
		return "", fmt.Errorf("ocr content is empty")
	}

	binaryPath, err := client.resolveBinary()
	if err != nil {
		return "", err
	}

	tempDir, err := os.MkdirTemp("", "ryuko-ocr-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input.png")
	if err := os.WriteFile(inputPath, content, 0o600); err != nil {
		return "", fmt.Errorf("write temp image: %w", err)
	}

	command := exec.CommandContext(
		ctx,
		binaryPath,
		inputPath,
		"stdout",
		"-l",
		client.language,
		"--psm",
		"6",
	)

	output, err := command.CombinedOutput()
	if err != nil {
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput == "" {
			return "", fmt.Errorf("run tesseract: %w", err)
		}
		return "", fmt.Errorf("run tesseract: %w: %s", err, trimmedOutput)
	}

	return strings.TrimSpace(string(output)), nil
}

func (client *TesseractClient) resolveBinary() (string, error) {
	if path, err := exec.LookPath(client.binaryPath); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("binary OCR %s tidak ditemukan", client.binaryPath)
}
