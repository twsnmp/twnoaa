package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"

	"twnoaa/pkg/ai/gemini"
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
	SatelliteName  string  `json:"satellite_name"`
	FrequencyMHz   float64 `json:"frequency_mhz"`
	TimestampStr   string  `json:"timestamp_str"`
	ObserverCity   string  `json:"observer_city"`
	ObserverLat    float64 `json:"observer_lat"`
	ObserverLon    float64 `json:"observer_lon"`
	MaxElevation   float64 `json:"max_elevation"`
	MaxElevationAz float64 `json:"max_elevation_az"`
	ImagePath      string  `json:"image_path"`
	AudioPath      string  `json:"audio_path"`
	Lang           string  `json:"lang"`
}

func initLLMModel(ctx context.Context, cfg datastore.Config) (llms.Model, error) {
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
		return ollama.New(
			ollama.WithServerURL(url),
			ollama.WithModel(mName),
		)
	case "openai":
		if cfg.APIKeyOpenAI == "" && strings.TrimSpace(cfg.OpenAIBaseURL) == "" {
			return nil, fmt.Errorf("OpenAI API key is missing in settings")
		}
		opts := []openai.Option{}
		if cfg.APIKeyOpenAI != "" {
			opts = append(opts, openai.WithToken(cfg.APIKeyOpenAI))
		} else {
			opts = append(opts, openai.WithToken("local-compatible"))
		}
		if baseURL := strings.TrimSpace(cfg.OpenAIBaseURL); baseURL != "" {
			opts = append(opts, openai.WithBaseURL(baseURL))
		}
		if mName := strings.TrimSpace(cfg.OpenAIModel); mName != "" {
			opts = append(opts, openai.WithModel(mName))
		}
		return openai.New(opts...)
	case "openrouter":
		if cfg.APIKeyOpenRouter == "" {
			return nil, fmt.Errorf("OpenRouter API key is missing in settings")
		}
		mName := strings.TrimSpace(cfg.OpenRouterModel)
		if mName == "" {
			mName = defaultOpenRouterModel
		}
		return openai.New(
			openai.WithToken(cfg.APIKeyOpenRouter),
			openai.WithBaseURL(openRouterBaseURL),
			openai.WithModel(mName),
		)
	case "gemini":
		if cfg.APIKeyGemini == "" {
			return nil, fmt.Errorf("Gemini API key is missing in settings")
		}
		mName := strings.TrimSpace(cfg.GeminiModel)
		if mName == "" {
			mName = defaultGeminiModel
		}
		return gemini.New(
			gemini.WithAPIKey(cfg.APIKeyGemini),
			gemini.WithModel(mName),
		)
	case "anthropic":
		if cfg.APIKeyAnthropic == "" {
			return nil, fmt.Errorf("Anthropic API key is missing in settings")
		}
		return anthropic.New(
			anthropic.WithToken(cfg.APIKeyAnthropic),
		)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.ActiveProvider)
	}
}

func AnalyzeSatelliteImage(ctx context.Context, cfg datastore.Config, req AnalysisRequest) (string, error) {
	model, err := initLLMModel(ctx, cfg)
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

	var systemPrompt string
	var userPrompt string

	if strings.ToLower(req.Lang) == "en" {
		systemPrompt = `You are an expert meteorologist analyzing a newly received APT (Automatic Picture Transmission) image from a NOAA weather satellite captured via an amateur RTL-SDR ground station.

Provide a clear, highly detailed, professional weather report strictly in English Markdown format.
Structure your report with:
1. **Reception Summary**: Satellite name, pass time, elevation, and ground observer location.
2. **Channel A & B Analysis**:
   - Channel A (Visible Light): Reflectivity, daytime cloud cover, coastal boundaries, and land surface details.
   - Channel B (Infrared): Cloud top temperatures, high-altitude cirrus clouds, and convection activity.
3. **Meteorological Features Identification**:
   - Low-pressure vortices, frontal systems, tropical depression structures, maritime cloud streets.
4. **Forecaster Notes & Practical Takeaways**.`

		userPrompt = fmt.Sprintf(`APT Satellite Reception Metadata:
- **Satellite**: %s
- **Frequency**: %.4f MHz
- **Timestamp**: %s
- **Location**: %s (Lat: %.3f°N, Lon: %.3f°E)
- **Max Elevation**: %.1f° (Azimuth: %.1f°)

Analyze the above reception metadata and attached image as a professional meteorologist, and generate a comprehensive meteorological analysis report in English Markdown.`,
			req.SatelliteName, req.FrequencyMHz, req.TimestampStr, req.ObserverCity, req.ObserverLat, req.ObserverLon, req.MaxElevation, req.MaxElevationAz)
	} else {
		systemPrompt = `You are an expert meteorologist analyzing a newly received APT (Automatic Picture Transmission) image from a NOAA weather satellite captured via an amateur RTL-SDR ground station.

Provide a clear, highly detailed, professional weather report in Japanese Markdown.
Structure your report with:
1. **受信サマリー (Reception Summary)**: Satellite name, pass time, elevation, and ground observer location.
2. **可視光・赤外線画像の解析 (Channel A & B Analysis)**:
   - Channel A (Visible Light): Reflectivity, daytime cloud cover, coastal boundaries, and land surface details.
   - Channel B (Infrared): Cloud top temperatures, high-altitude cirrus clouds, and convection activity.
3. **気象システムの特定 (Meteorological Features Identification)**:
   - Low-pressure vortices (低気圧の渦), frontal systems (温暖・寒冷前線), typhoon / tropical depression structures, maritime cloud streets.
4. **観測インサイト・予報コメント (Forecaster Notes & Practical Takeaways)**.`

		userPrompt = fmt.Sprintf(`気象衛星APT受信用メタデータ:
- **対象衛星名**: %s
- **中心周波数**: %.4f MHz
- **受信日時**: %s
- **観測地**: %s (緯度: %.3f°N, 経度: %.3f°E)
- **最大仰角 (Max Elevation)**: %.1f° (方位角: %.1f°)

上記の受信用データおよび添付画像を分析し、プロの気象予報士として日本語Markdown形式で詳細な画像解析結果を出力してください。`,
			req.SatelliteName, req.FrequencyMHz, req.TimestampStr, req.ObserverCity, req.ObserverLat, req.ObserverLon, req.MaxElevation, req.MaxElevationAz)
	}

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

const (
	openAITranscriptionURL = "https://api.openai.com/v1/audio/transcriptions"
	openRouterBaseURL      = "https://openrouter.ai/api/v1"
	defaultOpenRouterModel = "google/gemma-4-31b-it:free"
	defaultGeminiModel     = "gemini-2.5-flash"
)

// AnalyzeVoiceAudio transcribes and translates FM voice satellite audio (e.g. ISS repeater QSO)
// using Gemini multimodal audio input or OpenAI Whisper + chat completion.
func AnalyzeVoiceAudio(ctx context.Context, cfg datastore.Config, req AnalysisRequest) (string, error) {
	if req.AudioPath == "" {
		return "", fmt.Errorf("audio path is empty")
	}
	audioData, err := os.ReadFile(req.AudioPath)
	if err != nil {
		return "", fmt.Errorf("failed to read audio file: %w", err)
	}

	isEn := strings.ToLower(req.Lang) == "en"
	var systemPrompt, userPrompt string
	if isEn {
		systemPrompt = `You are an expert amateur radio operator and satellite communications analyst reviewing an FM voice audio recording received from a satellite (such as the ISS cross-band repeater) via an RTL-SDR ground station.

Provide a clear, accurate report strictly in English Markdown format.
Structure your report with:
1. **Reception Summary**: Satellite name, frequency, pass time, elevation, and observer location.
2. **Transcription**: Transcribe the spoken communications accurately, paying close attention to ITU phonetic alphabet callsigns (e.g., Romeo Zero Lima Bravo Charlie -> R0LBC), "CQ", and signal reports ("59").
3. **Identified Stations & Countries**: List each recognized callsign and identify its country/region based on the ITU prefix.
4. **Communication Summary**: Briefly summarize what took place during the pass.`

		userPrompt = fmt.Sprintf(`Satellite FM Voice Reception Metadata:
- **Satellite**: %s
- **Frequency**: %.4f MHz
- **Timestamp**: %s
- **Location**: %s (Lat: %.3f°N, Lon: %.3f°E)
- **Max Elevation**: %.1f° (Azimuth: %.1f°)

Listen carefully to the attached WAV audio, transcribe the amateur radio communications, identify callsigns/countries, and generate the report in English Markdown.`,
			req.SatelliteName, req.FrequencyMHz, req.TimestampStr, req.ObserverCity, req.ObserverLat, req.ObserverLon, req.MaxElevation, req.MaxElevationAz)
	} else {
		systemPrompt = `You are an expert amateur radio operator and satellite communications analyst reviewing an FM voice audio recording received from a satellite (such as the ISS cross-band repeater) via an RTL-SDR ground station.

Provide a clear, accurate report in Japanese Markdown format.
Structure your report with:
1. **受信サマリー (Reception Summary)**: Satellite name, frequency, pass time, elevation, and observer location.
2. **音声文字起こし (Transcription)**: Accurately transcribe the spoken communications (including ITU phonetic alphabet callsigns such as "Romeo Zero Lima Bravo Charlie" -> R0LBC, "CQ", and signal reports "59").
3. **日本語訳 (Translation)**: Translate the transcribed communications into natural Japanese.
4. **交信局・国地域の推定 (Stations & Regions)**: List each recognized callsign and explain its country/region based on the ITU prefix.`

		userPrompt = fmt.Sprintf(`衛星FM音声通信の受信メタデータ:
- **対象衛星名**: %s
- **中心周波数**: %.4f MHz
- **受信日時**: %s
- **観測地**: %s (緯度: %.3f°N, 経度: %.3f°E)
- **最大仰角 (Max Elevation)**: %.1f° (方位角: %.1f°)

添付のWAV音声を聴き取り、アマチュア無線の交信内容の文字起こし・日本語訳・コールサインと国地域の解説を日本語Markdown形式で出力してください。`,
			req.SatelliteName, req.FrequencyMHz, req.TimestampStr, req.ObserverCity, req.ObserverLat, req.ObserverLon, req.MaxElevation, req.MaxElevationAz)
	}

	switch cfg.ActiveProvider {
	case "gemini":
		model, err := initLLMModel(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("failed to initialize Gemini client: %w", err)
		}
		parts := []llms.ContentPart{
			llms.TextPart(userPrompt),
			llms.BinaryPart("audio/wav", audioData),
		}
		messages := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
			{
				Role:  llms.ChatMessageTypeHuman,
				Parts: parts,
			},
		}
		logDebug("Sending Gemini audio transcription request for %s (%d bytes)", req.SatelliteName, len(audioData))
		resp, err := model.GenerateContent(ctx, messages, llms.WithTemperature(0.2))
		if err != nil {
			return "", fmt.Errorf("LLM audio generation failed: %w", err)
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("empty response from LLM")
		}
		return resp.Choices[0].Content, nil

	case "openai":
		if cfg.APIKeyOpenAI == "" && strings.TrimSpace(cfg.OpenAIBaseURL) == "" {
			return "", fmt.Errorf("OpenAI API key is missing in settings")
		}
		transcriptionURL := openAITranscriptionURL
		if baseURL := strings.TrimRight(strings.TrimSpace(cfg.OpenAIBaseURL), "/"); baseURL != "" {
			transcriptionURL = baseURL + "/audio/transcriptions"
		}
		transcript, err := transcribeAudioWithOpenAIEndpoint(ctx, nil, transcriptionURL, cfg.APIKeyOpenAI, req.AudioPath, audioData)
		if err != nil {
			return "", fmt.Errorf("OpenAI audio transcription failed: %w", err)
		}
		model, err := initLLMModel(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("failed to initialize OpenAI chat client: %w", err)
		}
		combinedPrompt := fmt.Sprintf("%s\n\n### Raw Whisper Transcript:\n```\n%s\n```\n\nOrganize, refine callsigns/phonetics where obvious, translate, and format the final report according to the instructions.", userPrompt, transcript)
		messages := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
			llms.TextParts(llms.ChatMessageTypeHuman, combinedPrompt),
		}
		resp, err := model.GenerateContent(ctx, messages, llms.WithTemperature(0.2))
		if err != nil {
			return "", fmt.Errorf("LLM formatting failed: %w", err)
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("empty response from LLM")
		}
		return resp.Choices[0].Content, nil

	case "openrouter":
		if cfg.APIKeyOpenRouter == "" {
			return "", fmt.Errorf("OpenRouter API key is missing in settings")
		}
		mName := strings.TrimSpace(cfg.OpenRouterModel)
		if mName == "" {
			mName = defaultOpenRouterModel
		}
		return analyzeAudioWithOpenRouter(ctx, nil, openRouterBaseURL+"/chat/completions", cfg.APIKeyOpenRouter, mName, systemPrompt, userPrompt, audioData)

	case "anthropic", "ollama":
		return "", fmt.Errorf("audio transcription is currently supported with Gemini, OpenAI, or OpenRouter (audio-capable models) providers (active: %s)", cfg.ActiveProvider)
	default:
		return "", fmt.Errorf("unsupported AI provider: %s", cfg.ActiveProvider)
	}
}

func transcribeAudioWithOpenAIEndpoint(ctx context.Context, client *http.Client, endpoint, apiKey, audioPath string, audioData []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("model", "whisper-1"); err != nil {
		return "", err
	}
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audioData); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("invalid JSON response: %w", err)
	}
	return parsed.Text, nil
}

func analyzeAudioWithOpenRouter(ctx context.Context, client *http.Client, endpoint, apiKey, model, systemPrompt, userPrompt string, audioData []byte) (string, error) {
	payload := map[string]any{
		"model":       model,
		"temperature": 0.2,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": userPrompt,
					},
					{
						"type": "input_audio",
						"input_audio": map[string]string{
							"data":   base64.StdEncoding.EncodeToString(audioData),
							"format": "wav",
						},
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("invalid JSON response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty response from OpenRouter")
	}
	return parsed.Choices[0].Message.Content, nil
}

func formatAPRSPacketSummary(idx int, pkt datastore.APRSPacket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[#%d] Time: %s | Source: %s -> Dest: %s | Path: %s | Type: %s",
		idx,
		pkt.Timestamp.Format("2006-01-02 15:04:05"),
		pkt.SourceCall,
		pkt.DestCall,
		pkt.PathDisplay,
		pkt.PayloadType,
	)
	if pkt.HasLocation {
		fmt.Fprintf(&b, " | Lat/Lon: %.4f, %.4f", pkt.Latitude, pkt.Longitude)
	}
	if pkt.RSSI != 0 {
		fmt.Fprintf(&b, " | RSSI: %.1f dBFS", pkt.RSSI)
	}
	if pkt.Message != "" {
		fmt.Fprintf(&b, " | Message: %s", pkt.Message)
	}
	if pkt.Comment != "" {
		fmt.Fprintf(&b, " | Comment: %s", pkt.Comment)
	}
	if pkt.RawTNC != "" {
		fmt.Fprintf(&b, "\n    Raw TNC: %s", pkt.RawTNC)
	} else if pkt.RawPayload != "" {
		fmt.Fprintf(&b, "\n    Raw Payload: %s", pkt.RawPayload)
	}
	return b.String()
}

// AnalyzeAPRSPacket explains a single decoded APRS packet using the configured LLM.
func AnalyzeAPRSPacket(ctx context.Context, cfg datastore.Config, pkt datastore.APRSPacket, lang string) (string, error) {
	model, err := initLLMModel(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	var systemPrompt, userPrompt string
	pktSummary := formatAPRSPacketSummary(1, pkt)

	if strings.ToLower(lang) == "en" {
		systemPrompt = `You are an expert amateur radio operator and APRS (Automatic Packet Reporting System) / satellite packet communications analyst.

Explain the decoded APRS packet clearly in English Markdown format:
1. **Station & Callsign**: Identify the source callsign, SSID meaning, and likely country/region from its ITU prefix.
2. **Destination & Digipeater Path**: Explain the destination field (e.g. TOCALL, mic-e, or ISS/ARISS alias) and how the packet was relayed through the path.
3. **Payload & Content Analysis**: Decode and explain the payload type, coordinates/Maidenhead grid, message, telemetry, weather, or comment.
4. **Raw TNC Frame Breakdown**: Briefly walk through the raw AX.25/TNC2 frame structure.`

		userPrompt = fmt.Sprintf("Analyze and explain the following decoded APRS packet:\n\n%s", pktSummary)
	} else {
		systemPrompt = `You are an expert amateur radio operator and APRS (Automatic Packet Reporting System) / satellite packet communications analyst.

デコードされた APRS パケットの内容を日本語 Markdown 形式でわかりやすく解説してください。
構成:
1. **送信局・コールサイン**: コールサイン、SSID の意味、ITU プレフィックスに基づく国・地域の推定。
2. **宛先・中継経路 (PATH)**: 宛先フィールド（TOCALL 等）やデジピータ経路（RS0ISS / ARISS 衛星中継、WIDE1-1 等）の意味。
3. **ペイロード・本文の解説**: パケット種別、位置座標・グリッドロケーター、メッセージ、コメント、テレメトリ等の意味。
4. **生 TNC フレームの読み解き**: AX.25 / TNC2 形式の各要素の簡単な解説。`

		userPrompt = fmt.Sprintf("以下の受信 APRS パケットを解析し、日本語 Markdown 形式で解説してください。\n\n%s", pktSummary)
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	resp, err := model.GenerateContent(ctx, messages, llms.WithTemperature(0.2))
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}
	return resp.Choices[0].Content, nil
}

// AnalyzeAPRSPackets summarizes and explains all APRS packets captured during a satellite pass.
func AnalyzeAPRSPackets(ctx context.Context, cfg datastore.Config, req AnalysisRequest, packets []datastore.APRSPacket) (string, error) {
	if len(packets) == 0 {
		return "", fmt.Errorf("no APRS packets available for analysis")
	}

	model, err := initLLMModel(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	const maxPacketsInPrompt = 100
	limited := packets
	truncatedNote := ""
	if len(packets) > maxPacketsInPrompt {
		limited = packets[:maxPacketsInPrompt]
		truncatedNote = fmt.Sprintf("\n(Showing first %d of %d total packets)", maxPacketsInPrompt, len(packets))
	}

	var listBuilder strings.Builder
	for i, pkt := range limited {
		listBuilder.WriteString(formatAPRSPacketSummary(i+1, pkt))
		listBuilder.WriteString("\n")
	}

	var systemPrompt, userPrompt string
	if strings.ToLower(req.Lang) == "en" {
		systemPrompt = `You are an expert amateur radio operator and satellite packet communications analyst reviewing APRS packets received from an amateur radio satellite (such as the ISS ARISS digipeater) via an RTL-SDR ground station.

Provide a clear, detailed report in English Markdown format:
1. **Reception Summary**: Satellite name, frequency, pass time, elevation, observer location, and total packet count.
2. **Active Stations & Regions**: List unique source callsigns, identify their ITU countries/regions, and summarize activity.
3. **Digipeater & Relay Paths**: Explain how stations routed packets through the satellite digipeater (e.g. ARISS, RS0ISS, direct vs relayed).
4. **Messages, Telemetry & Notable Packets**: Highlight interesting QSO messages, position reports, Maidenhead grids, or status/telemetry beacons.`

		userPrompt = fmt.Sprintf(`Satellite APRS Reception Metadata:
- **Satellite**: %s
- **Frequency**: %.4f MHz
- **Timestamp**: %s
- **Location**: %s (Lat: %.3f°N, Lon: %.3f°E)
- **Max Elevation**: %.1f° (Azimuth: %.1f°)
- **Total Packets**: %d%s

### Decoded APRS Packets:
%s

Analyze the above pass metadata and decoded APRS packets, and generate a comprehensive report in English Markdown.`,
			req.SatelliteName, req.FrequencyMHz, req.TimestampStr, req.ObserverCity, req.ObserverLat, req.ObserverLon, req.MaxElevation, req.MaxElevationAz, len(packets), truncatedNote, listBuilder.String())
	} else {
		systemPrompt = `You are an expert amateur radio operator and satellite packet communications analyst reviewing APRS packets received from an amateur radio satellite (such as the ISS ARISS digipeater) via an RTL-SDR ground station.

日本語 Markdown 形式で分かりやすく詳細な解析レポートを作成してください。
構成:
1. **受信サマリー**: 対象衛星名、周波数、受信日時、最大仰角、観測地、受信パケット総数。
2. **確認された送信局と国・地域**: ユニークなコールサイン一覧と ITU プレフィックスに基づく国・地域の解説。
3. **デジピータ中継経路の傾向**: ARISS / RS0ISS 経由の中継状況や PATH の特徴。
4. **メッセージ・位置情報・注目パケットの解説**: 交信メッセージ、グリッドロケーター・座標、ステータスやテレメトリの解説。`

		userPrompt = fmt.Sprintf(`衛星 APRS パケット受信メタデータ:
- **対象衛星名**: %s
- **中心周波数**: %.4f MHz
- **受信日時**: %s
- **観測地**: %s (緯度: %.3f°N, 経度: %.3f°E)
- **最大仰角**: %.1f° (方位角: %.1f°)
- **受信パケット数**: %d 件%s

### デコード済み APRS パケット一覧:
%s

上記の受信データと APRS パケット一覧を解析し、日本語 Markdown 形式で詳細なレポートを出力してください。`,
			req.SatelliteName, req.FrequencyMHz, req.TimestampStr, req.ObserverCity, req.ObserverLat, req.ObserverLon, req.MaxElevation, req.MaxElevationAz, len(packets), truncatedNote, listBuilder.String())
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	resp, err := model.GenerateContent(ctx, messages, llms.WithTemperature(0.2))
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}
	return resp.Choices[0].Content, nil
}
