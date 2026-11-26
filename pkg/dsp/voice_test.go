package dsp

import (
	"math"
	"math/rand"
	"path/filepath"
	"testing"
)

func TestExtractVoiceSegmentsAndEnhance(t *testing.T) {
	sampleRate := 20800
	durationSec := 12
	total := sampleRate * durationSec
	samples := make([]float64, total)

	rng := rand.New(rand.NewSource(42))
	// Fill with high-frequency FM static noise (amplitude ~0.4)
	for i := 0; i < total; i++ {
		samples[i] = (rng.Float64()*2 - 1) * 0.4
	}

	// Insert a clear 1kHz speech-band tone (quieting + low HF ratio) between 4s and 7s
	for i := sampleRate * 4; i < sampleRate*7; i++ {
		samples[i] = 0.15 * math.Sin(2*math.Pi*1000*float64(i)/float64(sampleRate))
	}

	extracted, ok := ExtractVoiceSegments(samples, sampleRate)
	if !ok {
		t.Fatalf("expected ExtractVoiceSegments to detect voice segment")
	}
	// 3s voice + 1.5s padding on each side = ~6s (much shorter than 12s)
	if len(extracted) >= total {
		t.Fatalf("expected extracted length (%d) to be shorter than original (%d)", len(extracted), total)
	}
	if len(extracted) < sampleRate*3 {
		t.Fatalf("expected extracted length (%d) to cover at least 3s voice", len(extracted))
	}

	enhanced := EnhanceVoiceAudio(extracted, sampleRate)
	if len(enhanced) != len(extracted) {
		t.Fatalf("expected enhanced length %d, got %d", len(extracted), len(enhanced))
	}
	for i, v := range enhanced {
		if v > 1.0 || v < -1.0 || math.IsNaN(v) {
			t.Fatalf("invalid enhanced sample at %d: %v", i, v)
		}
	}
}

func TestProcessFMVoiceWAV(t *testing.T) {
	sampleRate := 20800
	samples := make([]float64, sampleRate*8)
	rng := rand.New(rand.NewSource(7))
	for i := range samples {
		samples[i] = (rng.Float64()*2 - 1) * 0.35
	}
	for i := sampleRate * 3; i < sampleRate*5; i++ {
		samples[i] = 0.2 * math.Sin(2*math.Pi*800*float64(i)/float64(sampleRate))
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src.wav")
	dst := filepath.Join(dir, "dst.wav")

	if err := SaveWAV(src, sampleRate, ConvertFloatToINT16(samples)); err != nil {
		t.Fatalf("SaveWAV failed: %v", err)
	}

	extracted, err := ProcessFMVoiceWAV(src, dst)
	if err != nil {
		t.Fatalf("ProcessFMVoiceWAV failed: %v", err)
	}
	if !extracted {
		t.Fatalf("expected extracted=true")
	}

	loaded, sr, err := LoadWAV(dst)
	if err != nil {
		t.Fatalf("LoadWAV dst failed: %v", err)
	}
	if sr != sampleRate {
		t.Fatalf("expected sampleRate %d, got %d", sampleRate, sr)
	}
	if len(loaded) == 0 || len(loaded) >= len(samples) {
		t.Fatalf("unexpected dst length: %d (src %d)", len(loaded), len(samples))
	}
}
