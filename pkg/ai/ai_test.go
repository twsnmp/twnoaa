package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"twnoaa/pkg/datastore"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestTranscribeAudioWithOpenAIEndpoint(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("unexpected Authorization header: %s", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"text":"CQ satellite Romeo Zero Lima Bravo Charlie"}`)),
			}, nil
		}),
	}

	txt, err := transcribeAudioWithOpenAIEndpoint(context.Background(), client, "https://api.openai.com/v1/audio/transcriptions", "test-key", "sample.wav", []byte("RIFFfake"))
	if err != nil {
		t.Fatalf("transcribeAudioWithOpenAIEndpoint failed: %v", err)
	}
	if txt != "CQ satellite Romeo Zero Lima Bravo Charlie" {
		t.Fatalf("unexpected transcript: %s", txt)
	}
}

func TestAnalyzeVoiceAudioValidation(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "voice.wav")
	if err := os.WriteFile(tmp, []byte("RIFFfake"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := AnalyzeVoiceAudio(context.Background(), datastore.Config{ActiveProvider: "ollama"}, AnalysisRequest{
		AudioPath: tmp,
	})
	if err == nil {
		t.Fatalf("expected error for unsupported provider ollama")
	}

	_, err = AnalyzeVoiceAudio(context.Background(), datastore.Config{ActiveProvider: "gemini"}, AnalysisRequest{
		AudioPath: "",
	})
	if err == nil {
		t.Fatalf("expected error for empty audio path")
	}
}

func TestInitLLMModelProviders(t *testing.T) {
	ctx := context.Background()

	if _, err := initLLMModel(ctx, datastore.Config{ActiveProvider: "openrouter"}); err == nil {
		t.Fatalf("expected error when OpenRouter API key is missing")
	}
	if _, err := initLLMModel(ctx, datastore.Config{
		ActiveProvider:   "openrouter",
		APIKeyOpenRouter: "sk-or-test",
		OpenRouterModel:  "google/gemma-4-31b-it:free",
	}); err != nil {
		t.Fatalf("unexpected error for openrouter provider: %v", err)
	}
	if _, err := initLLMModel(ctx, datastore.Config{
		ActiveProvider: "openai",
		OpenAIBaseURL:  "http://localhost:1234/v1",
		OpenAIModel:    "local-model",
	}); err != nil {
		t.Fatalf("unexpected error for openai-compatible provider without API key: %v", err)
	}
	if _, err := initLLMModel(ctx, datastore.Config{
		ActiveProvider: "gemini",
		APIKeyGemini:   "AIzaTest",
		GeminiModel:    "gemini-2.5-flash",
	}); err != nil {
		t.Fatalf("unexpected error for gemini provider: %v", err)
	}
}

func TestAnalyzeAudioWithOpenRouter(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer sk-or-test" {
				t.Fatalf("unexpected Authorization header: %s", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode request JSON: %v", err)
			}
			if body["model"] != "google/gemini-2.5-flash" {
				t.Fatalf("unexpected model: %v", body["model"])
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"QSO transcript OK"}}]}`)),
			}, nil
		}),
	}

	out, err := analyzeAudioWithOpenRouter(context.Background(), client, "https://openrouter.ai/api/v1/chat/completions", "sk-or-test", "google/gemini-2.5-flash", "sys", "usr", []byte("RIFFfake"))
	if err != nil {
		t.Fatalf("analyzeAudioWithOpenRouter failed: %v", err)
	}
	if out != "QSO transcript OK" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestAnalyzeAPRSPacketsValidationAndFormat(t *testing.T) {
	if _, err := AnalyzeAPRSPackets(context.Background(), datastore.Config{ActiveProvider: "ollama"}, AnalysisRequest{}, nil); err == nil {
		t.Fatalf("expected error for empty APRS packets")
	}

	pkt := datastore.APRSPacket{
		Timestamp:   time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC),
		SourceCall:  "JA1XYZ-7",
		DestCall:    "CQ",
		PathDisplay: "RS0ISS*",
		PayloadType: "position",
		HasLocation: true,
		Latitude:    35.6895,
		Longitude:   139.6917,
		Comment:     "Hello via ISS",
		RawTNC:      "JA1XYZ-7>CQ,RS0ISS*:!3541.37N/13941.50E>Hello via ISS",
		RSSI:        -18.5,
	}
	summary := formatAPRSPacketSummary(1, pkt)
	if !strings.Contains(summary, "JA1XYZ-7") || !strings.Contains(summary, "RS0ISS*") || !strings.Contains(summary, "35.6895") {
		t.Fatalf("unexpected formatted APRS summary: %s", summary)
	}
}
