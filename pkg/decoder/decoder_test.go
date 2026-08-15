package decoder

import (
	"math"
	"os"
	"testing"

	"twnoaa/pkg/dsp"
)

func TestDecodeWAVFile(t *testing.T) {
	sampleRates := []int{20800, 11025, 44100}

	for _, sampleRate := range sampleRates {
		tempDir := t.TempDir()
		wavPath := tempDir + "/sample_apt.wav"

		// Create 2 seconds of synthetic APT audio at target sample rate (4 lines of APT)
		durationSec := 2.0
		numSamples := int(float64(sampleRate) * durationSec)
		samples := make([]float64, numSamples)

		for i := 0; i < numSamples; i++ {
			tSec := float64(i) / float64(sampleRate)

			// Line period is 0.5s
			lineTime := math.Mod(tSec, 0.5)
			var baseVal float64

			if lineTime < 0.05 {
				// Sync A pulse (7 cycles of 1040 Hz square wave)
				syncPhase := math.Mod(lineTime*1040.0, 1.0)
				if syncPhase < 0.5 {
					baseVal = 1.0
				} else {
					baseVal = 0.0
				}
			} else {
				// Image video signal modulated on 2400 Hz subcarrier
				baseVal = 0.5 + 0.4*math.Sin(2.0*math.Pi*2.0*tSec)
			}

			// AM modulation on 2400 Hz subcarrier
			subcarrier := math.Sin(2.0 * math.Pi * 2400.0 * tSec)
			samples[i] = baseVal * subcarrier
		}

		int16Audio := dsp.ConvertFloatToINT16(samples)
		if err := dsp.SaveWAV(wavPath, sampleRate, int16Audio); err != nil {
			t.Fatalf("Failed to save synthetic WAV file at %d Hz: %v", sampleRate, err)
		}

		outDir := tempDir + "/output_images"
		baseName := "test_wav_decode"

		res, err := DecodeWAVFile(wavPath, outDir, baseName)
		if err != nil {
			t.Fatalf("DecodeWAVFile failed for sampleRate=%d: %v", sampleRate, err)
		}

		if res == nil {
			t.Fatalf("DecodeWAVFile returned nil result for sampleRate=%d", sampleRate)
		}

		if res.LineCount != 4 {
			t.Errorf("Expected 4 lines decoded for sampleRate=%d, got %d", sampleRate, res.LineCount)
		}

		if _, err := os.Stat(res.FullImagePath); os.IsNotExist(err) {
			t.Errorf("Full image file was not created for sampleRate=%d: %s", sampleRate, res.FullImagePath)
		}
		if _, err := os.Stat(res.ChanAPath); os.IsNotExist(err) {
			t.Errorf("ChanA image file was not created for sampleRate=%d: %s", sampleRate, res.ChanAPath)
		}
		if _, err := os.Stat(res.ChanBPath); os.IsNotExist(err) {
			t.Errorf("ChanB image file was not created for sampleRate=%d: %s", sampleRate, res.ChanBPath)
		}
	}
}
