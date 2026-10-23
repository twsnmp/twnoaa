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

func TestDecodeSSTV_Robot36(t *testing.T) {
	sampleRate := 20800
	durationSec := 36.0 // Full 240 lines of Robot 36 (240 * 0.15s = 36s)
	numSamples := int(float64(sampleRate) * durationSec)
	samples := make([]float64, numSamples)

	var phase float64
	for i := 0; i < numSamples; i++ {
		tSec := float64(i) / float64(sampleRate)
		lineIdx := int(tSec/0.150) % 240
		lineTime := math.Mod(tSec, 0.150)

		var toneHz float64
		if lineTime < 0.009 {
			toneHz = 1200.0 // Sync pulse
		} else if lineTime < 0.012 {
			toneHz = 1500.0 // Sync porch
		} else if lineTime < 0.100 {
			// Y scan (88 ms)
			xNorm := (lineTime - 0.012) / 0.088
			yNorm := float64(lineIdx) / 240.0
			var luma float64
			if yNorm < 0.22 {
				barIdx := int(xNorm * 7.0)
				barsY := []float64{0.92, 0.81, 0.65, 0.54, 0.41, 0.30, 0.15}
				if barIdx >= 0 && barIdx < len(barsY) {
					luma = barsY[barIdx]
				}
			} else {
				earthHorizon := 0.58 + 0.10*math.Sin(xNorm*math.Pi)
				dxSolar1 := math.Abs(xNorm - 0.30)
				dxSolar2 := math.Abs(xNorm - 0.70)
				dySolar := math.Abs(yNorm - 0.42)
				if (dxSolar1 < 0.10 || dxSolar2 < 0.10) && dySolar < 0.14 {
					luma = 0.72 + 0.18*math.Sin(yNorm*120.0)
				} else if math.Abs(xNorm-0.50) < 0.12 && math.Abs(yNorm-0.42) < 0.05 {
					luma = 0.88
				} else if yNorm > earthHorizon {
					cloud := 0.5 + 0.5*math.Sin(xNorm*18.0+yNorm*12.0)*math.Cos(xNorm*9.0-yNorm*15.0)
					luma = 0.42 + 0.48*cloud
				} else {
					luma = 0.08
				}
			}
			toneHz = 1500.0 + luma*800.0
		} else if lineTime < 0.1045 {
			if lineIdx%2 == 0 {
				toneHz = 1500.0
			} else {
				toneHz = 2300.0
			}
		} else if lineTime < 0.106 {
			toneHz = 1900.0
		} else {
			xNorm := (lineTime - 0.106) / 0.044
			yNorm := float64(lineIdx) / 240.0
			earthHorizon := 0.58 + 0.10*math.Sin(xNorm*math.Pi)
			var chroma float64 = 0.5
			if yNorm < 0.22 {
				barIdx := int(xNorm * 7.0)
				if lineIdx%2 == 0 {
					barsCr := []float64{0.50, 0.56, 0.10, 0.16, 0.84, 0.90, 0.44}
					if barIdx >= 0 && barIdx < len(barsCr) {
						chroma = barsCr[barIdx]
					}
				} else {
					barsCb := []float64{0.50, 0.10, 0.65, 0.25, 0.75, 0.35, 0.90}
					if barIdx >= 0 && barIdx < len(barsCb) {
						chroma = barsCb[barIdx]
					}
				}
			} else {
				dxSolar1 := math.Abs(xNorm - 0.30)
				dxSolar2 := math.Abs(xNorm - 0.70)
				dySolar := math.Abs(yNorm - 0.42)
				if (dxSolar1 < 0.10 || dxSolar2 < 0.10) && dySolar < 0.14 {
					if lineIdx%2 == 0 {
						chroma = 0.68
					} else {
						chroma = 0.28
					}
				} else if yNorm > earthHorizon {
					cloud := 0.5 + 0.5*math.Sin(xNorm*18.0+yNorm*12.0)*math.Cos(xNorm*9.0-yNorm*15.0)
					if lineIdx%2 == 0 {
						chroma = 0.22 + 0.28*cloud
					} else {
						chroma = 0.82 - 0.28*cloud
					}
				}
			}
			toneHz = 1500.0 + chroma*800.0
		}

		phase += 2.0 * math.Pi * toneHz / float64(sampleRate)
		if phase > 2.0*math.Pi {
			phase -= 2.0 * math.Pi
		}
		samples[i] = 0.8 * math.Sin(phase)
	}

	outDir := t.TempDir()
	res, err := DecodeSSTV(samples, sampleRate, outDir, "test_iss_sstv")
	if err != nil {
		t.Fatalf("DecodeSSTV failed: %v", err)
	}
	if res == nil || res.LineCount < 100 {
		t.Fatalf("Expected >= 100 SSTV lines, got %+v", res)
	}
	if _, err := os.Stat(res.FullImagePath); os.IsNotExist(err) {
		t.Fatalf("SSTV image was not created at %s", res.FullImagePath)
	}
}

