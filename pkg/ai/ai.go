package ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"


	"twnoaa/pkg/datastore"
)

var debugMode = func() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("TWNOAA_DEBUG")))
	return v == "1" || v == "true"
}()

func logDebug(format string, args ...any) {
	if debugMode {
		log.Printf(format, args...)
	}
}

type AnalysisRequest struct {
	SatelliteName string  `json:"satellite_name"`
	FrequencyMHz  float64 `json:"frequency_mhz"`
	TimestampStr  string  `json:"timestamp_str"`
	ObserverCity  string  `json:"observer_city"`
	ObserverLat   float64 `json:"observer_lat"`
	ObserverLon   float64 `json:"observer_lon"`
	MaxElevation  float64 `json:"max_elevation"`
	ImagePath     string  `json:"image_path"`
}

func AnalyzeSatelliteImage(ctx context.Context, cfg datastore.Config, req AnalysisRequest) (string, error) {
	var model llms.Model
	var err error

	switch cfg.ActiveProvider {
	case "ollama":
		url := cfg.OllamaURL
		if url == "" {
			url = "http://localhost:11434"
		}
		mName := cfg.OllamaModel
		if mName == "" {
			mName = "llama3"
		}
		model, err = ollama.New(
			ollama.WithServerURL(url),
			ollama.WithModel(mName),
		)
	case "openai":
		if cfg.APIKeyOpenAI == "" {
			return "", fmt.Errorf("OpenAI API key is missing in settings")
		}
		model, err = openai.New(
			openai.WithToken(cfg.APIKeyOpenAI),
		)
	case "gemini":
		if cfg.APIKeyGemini == "" {
			return "", fmt.Errorf("Gemini API key is missing in settings")
		}
		model, err = googleai.New(
			ctx,
			googleai.WithAPIKey(cfg.APIKeyGemini),
		)
	case "anthropic":
		if cfg.APIKeyAnthropic == "" {
			return "", fmt.Errorf("Anthropic API key is missing in settings")
		}
		model, err = anthropic.New(
			anthropic.WithToken(cfg.APIKeyAnthropic),
		)
	default:
		return "", fmt.Errorf("unsupported AI provider: %s", cfg.ActiveProvider)
	}

	if err != nil {
		return "", fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Read image file if exists
	var imgData []byte
	if req.ImagePath != "" {
		if data, err := os.ReadFile(req.ImagePath); err == nil {
			imgData = data
		}
	}

	systemPrompt := `You are an expert meteorologist analyzing a newly received APT (Automatic Picture Transmission) image from a NOAA weather satellite captured via an amateur RTL-SDR ground station.

Provide a clear, highly detailed, professional weather report in Japanese Markdown.
Structure your report with:
1. **受信サマリー (Reception Summary)**: Satellite name, pass time, elevation, and ground observer location.
2. **可視光・赤外線画像の解析 (Channel A & B Analysis)**:
   - Channel A (Visible Light): Reflectivity, daytime cloud cover, coastal boundaries, and land surface details.
   - Channel B (Infrared): Cloud top temperatures, high-altitude cirrus clouds, and convection activity.
3. **気象システムの特定 (Meteorological Features Identification)**:
   - Low-pressure vortices (低気圧の渦), frontal systems (温暖・寒冷前線), typhoon / tropical depression structures, maritime cloud streets.
4. **観測インサイト・予報コメント (Forecaster Notes & Practical Takeaways)**.`

	userPrompt := fmt.Sprintf(`気象衛星APT受信用メタデータ:
- **対象衛星名**: %s
- **中心周波数**: %.4f MHz
- **受信日時**: %s
- **観測地**: %s (緯度: %.3f°N, 経度: %.3f°E)
- **最大仰角 (Max Elevation)**: %.1f°

上記の受信用データおよび添付画像を分析し、プロの気象予報士として日本語Markdown形式で詳細な画像解析結果を出力してください。`,
		req.SatelliteName, req.FrequencyMHz, req.TimestampStr, req.ObserverCity, req.ObserverLat, req.ObserverLon, req.MaxElevation)

	parts := []llms.ContentPart{
		llms.TextPart(userPrompt),
	}

	if len(imgData) > 0 {
		parts = append(parts, llms.BinaryPart("image/png", imgData))
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: parts,
		},
	}

	logDebug("Sending LLM request for satellite %s image %s", req.SatelliteName, req.ImagePath)

	resp, err := model.GenerateContent(ctx, messages, llms.WithTemperature(0.2))
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	analysisText := resp.Choices[0].Content
	return analysisText, nil
}
