package dsp

import (
	"math"
	"testing"
)

func TestIQToComplexAndDemodulation(t *testing.T) {
	// Synthesize 8-bit IQ bytes
	buf := make([]byte, 200)
	for i := 0; i < 100; i++ {
		angle := float64(i) * 0.1
		buf[2*i] = byte((math.Cos(angle) + 1.0) * 127.5)
		buf[2*i+1] = byte((math.Sin(angle) + 1.0) * 127.5)
	}

	complexSamples := ConvertIQToComplex(buf)
	if len(complexSamples) != 100 {
		t.Fatalf("Expected 100 complex samples, got %d", len(complexSamples))
	}

	dspEngine := NewDSP(2080000, 20800)
	audio := dspEngine.DemodulateFM(complexSamples)
	if len(audio) == 0 {
		t.Fatalf("Expected non-empty audio array")
	}

	spectrum := CalculatePowerSpectrum(buf, 64)
	if len(spectrum) != 64 {
		t.Fatalf("Expected 64 spectrum bins, got %d", len(spectrum))
	}
}
