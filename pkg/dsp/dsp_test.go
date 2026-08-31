package dsp

import (
	"math"
	"os"
	"testing"
)

func TestIQToComplexAndDemodulation(t *testing.T) {
	// Synthesize 8-bit IQ bytes with decimation depth (2000 samples = 1000 IQ pairs)
	buf := make([]byte, 2000)
	for i := 0; i < 1000; i++ {
		angle := float64(i) * 0.1
		buf[2*i] = byte((math.Cos(angle) + 1.0) * 127.5)
		buf[2*i+1] = byte((math.Sin(angle) + 1.0) * 127.5)
	}

	complexSamples := ConvertIQToComplex(buf)
	if len(complexSamples) != 1000 {
		t.Fatalf("Expected 1000 complex samples, got %d", len(complexSamples))
	}

	dspEngine := NewDSP(2080000, 20800)
	audio := dspEngine.DemodulateFM(complexSamples)
	if len(audio) != 10 {
		t.Fatalf("Expected 10 audio samples (1000 / 100), got %d", len(audio))
	}

	spectrum := CalculatePowerSpectrum(buf, 64)
	if len(spectrum) != 64 {
		t.Fatalf("Expected 64 spectrum bins, got %d", len(spectrum))
	}
}

func TestCalculateRSSIWithDCOffset(t *testing.T) {
	// 1. Constant hardware DC offset with zero AC signal (all bytes = 180)
	dcBuf := make([]byte, 2000)
	for i := 0; i < len(dcBuf); i++ {
		dcBuf[i] = 180
	}

	rssiDC := CalculateRSSI(dcBuf)
	if rssiDC > -80.0 {
		t.Errorf("Expected RSSI for pure DC offset to be <= -80 dBFS, got %.2f dBFS", rssiDC)
	}

	// 2. AC signal added on top of DC offset
	acBuf := make([]byte, 2000)
	for i := 0; i < 1000; i++ {
		valI := 0.2 * math.Cos(float64(i)*0.2) // AC signal amplitude 0.2
		valQ := 0.2 * math.Sin(float64(i)*0.2)
		acBuf[2*i] = byte(math.Max(0, math.Min(255, (valI+0.3+1.0)*127.5))) // +0.3 DC bias
		acBuf[2*i+1] = byte(math.Max(0, math.Min(255, (valQ+0.3+1.0)*127.5)))
	}

	rssiAC := CalculateRSSI(acBuf)
	// Expected AC RMS power: (0.2^2 + 0.2^2)/2 = 0.04 -> 10*log10(0.04) = -13.98 dBFS
	if rssiAC < -20.0 || rssiAC > -10.0 {
		t.Errorf("Expected RSSI for AC signal to be around -14 dBFS, got %.2f dBFS", rssiAC)
	}
}

func TestDemodulateFMWithDCOffsetAndSubcarrier(t *testing.T) {
	sampleRate := 2080000
	targetRate := 20800
	numSamples := sampleRate / 10 // 0.1 sec = 208,000 samples

	iq := make([]complex128, numSamples)
	phase := 0.0
	fSub := 2400.0
	maxDev := 17000.0

	for i := 0; i < numSamples; i++ {
		tSec := float64(i) / float64(sampleRate)
		subcarrier := math.Sin(2.0 * math.Pi * fSub * tSec)
		instFreq := maxDev * subcarrier
		phase += 2.0 * math.Pi * instFreq / float64(sampleRate)

		// Strong DC offset +0.4 on I and Q
		r := math.Cos(phase) + 0.4
		im := math.Sin(phase) + 0.4
		iq[i] = complex(r, im)
	}

	dspEngine := NewDSP(sampleRate, targetRate)
	audio := dspEngine.DemodulateFM(iq)

	if len(audio) != numSamples/100 {
		t.Fatalf("Expected %d audio samples, got %d", numSamples/100, len(audio))
	}

	// Verify audio is non-zero and has non-trivial dynamic range (not locked to 0)
	var maxVal, minVal float64
	for _, v := range audio {
		if v > maxVal {
			maxVal = v
		}
		if v < minVal {
			minVal = v
		}
	}

	if maxVal-minVal < 0.2 {
		t.Errorf("Demodulated audio range too small (max=%.3f, min=%.3f), FM discriminator may be captured by DC", maxVal, minVal)
	}
}

func TestSaveAndLoadWAV(t *testing.T) {
	tempFile := t.TempDir() + "/test_roundtrip.wav"

	// Generate 1 second of 440Hz sine wave
	sampleRate := 20800
	samples := make([]float64, sampleRate)
	for i := 0; i < sampleRate; i++ {
		tSec := float64(i) / float64(sampleRate)
		samples[i] = 0.8 * math.Sin(2.0*math.Pi*440.0*tSec)
	}

	int16Samples := ConvertFloatToINT16(samples)
	if err := SaveWAV(tempFile, sampleRate, int16Samples); err != nil {
		t.Fatalf("SaveWAV failed: %v", err)
	}

	loadedSamples, loadedRate, err := LoadWAV(tempFile)
	if err != nil {
		t.Fatalf("LoadWAV failed: %v", err)
	}

	if loadedRate != sampleRate {
		t.Errorf("Expected sample rate %d, got %d", sampleRate, loadedRate)
	}

	if len(loadedSamples) != len(samples) {
		t.Errorf("Expected %d samples, got %d", len(samples), len(loadedSamples))
	}

	// Compare sample values with small tolerance for 16-bit quantization
	for i := 0; i < len(samples); i++ {
		diff := math.Abs(samples[i] - loadedSamples[i])
		if diff > 0.001 {
			t.Errorf("Sample mismatch at index %d: original=%.5f, loaded=%.5f (diff=%.5f)", i, samples[i], loadedSamples[i], diff)
			break
		}
	}
}

func TestAnalyzeRawIQSpectrumAndWAVSpectrum(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Test AnalyzeRawIQSpectrum
	iqPath := tempDir + "/test_raw.iq"
	iqBuf := make([]byte, 208000*2) // 0.1 sec at 2.08 MSps
	for i := 0; i < 208000; i++ {
		angle := float64(i) * 0.05
		iqBuf[2*i] = byte((math.Cos(angle) + 1.0) * 127.5)
		iqBuf[2*i+1] = byte((math.Sin(angle) + 1.0) * 127.5)
	}
	if err := os.WriteFile(iqPath, iqBuf, 0644); err != nil {
		t.Fatalf("Failed to write test IQ file: %v", err)
	}

	iqSpec, err := AnalyzeRawIQSpectrum(iqPath, 137100000, 2080000, 128, 50)
	if err != nil {
		t.Fatalf("AnalyzeRawIQSpectrum failed: %v", err)
	}
	if iqSpec.Type != "rf_iq" {
		t.Errorf("Expected Type rf_iq, got %s", iqSpec.Type)
	}
	if len(iqSpec.Frames) == 0 {
		t.Fatalf("Expected non-empty frames, got 0")
	}
	if len(iqSpec.Frames[0].Bins) != 128 || len(iqSpec.Frames[0].BinsDb) != 128 {
		t.Errorf("Expected 128 bins in frame, got %d and %d", len(iqSpec.Frames[0].Bins), len(iqSpec.Frames[0].BinsDb))
	}

	// 2. Test AnalyzeWAVSpectrum
	wavPath := tempDir + "/test_audio.wav"
	wavSamples := make([]float64, 20800) // 1 sec at 20.8 kHz
	for i := 0; i < 20800; i++ {
		tSec := float64(i) / 20800.0
		wavSamples[i] = 0.7 * math.Sin(2.0*math.Pi*2400.0*tSec) // 2.4 kHz AM carrier
	}
	if err := SaveAudioToWAV(wavPath, 20800, wavSamples); err != nil {
		t.Fatalf("Failed to write test WAV: %v", err)
	}

	wavSpec, err := AnalyzeWAVSpectrum(wavPath, 128, 50)
	if err != nil {
		t.Fatalf("AnalyzeWAVSpectrum failed: %v", err)
	}
	if wavSpec.Type != "audio_wav" {
		t.Errorf("Expected Type audio_wav, got %s", wavSpec.Type)
	}
	if len(wavSpec.Frames) == 0 {
		t.Fatalf("Expected non-empty frames for WAV, got 0")
	}
	if len(wavSpec.Frames[0].Bins) != 128 || len(wavSpec.Frames[0].BinsDb) != 128 {
		t.Errorf("Expected 128 bins in WAV frame, got %d and %d", len(wavSpec.Frames[0].Bins), len(wavSpec.Frames[0].BinsDb))
	}
}



