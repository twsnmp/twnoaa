package decoder

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"sort"

	"twnoaa/pkg/dsp"
	"twnoaa/pkg/logger"
)

const TargetWidth = 2080

type DecodeResult struct {
	FullImagePath string `json:"full_image_path"`
	ChanAPath     string `json:"chan_a_path"`
	ChanBPath     string `json:"chan_b_path"`
	LineCount     int    `json:"line_count"`
	SyncOffset    int    `json:"sync_offset"`
}

// DecodeAPT processes demodulated audio float64 samples (-1.0 to 1.0) and generates APT satellite images
func DecodeAPT(samples []float64, sampleRate int, outDir, baseName string) (*DecodeResult, error) {
	if len(samples) == 0 {
		logger.Errorf("DECODER", "DecodeAPT failed: no audio samples provided")
		return nil, fmt.Errorf("no audio samples provided")
	}

	durationSec := float64(len(samples)) / float64(sampleRate)
	logger.Infof("DECODER", "Starting APT decode: baseName=%s, total_samples=%d (duration=%.1fs), sample_rate=%dHz", baseName, len(samples), durationSec, sampleRate)

	// Ensure output directory exists
	if err := os.MkdirAll(outDir, 0755); err != nil {
		logger.Errorf("DECODER", "Failed to create output directory %s: %v", outDir, err)
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	// 1. Extract AM envelope
	envelope := extractEnvelope(samples, sampleRate)

	// 2. Slice into 0.5s line chunks and resample to 2080 px TargetWidth
	samplesPerLine := int(float64(sampleRate) * 0.5)
	if samplesPerLine <= 0 {
		samplesPerLine = 10400 // fallback for 20800 Hz
	}

	var rawFloatLines [][]float64
	for i := 0; i+samplesPerLine <= len(envelope); i += samplesPerLine {
		line := resampleLine(envelope[i:i+samplesPerLine], TargetWidth)
		rawFloatLines = append(rawFloatLines, line)
	}

	if len(rawFloatLines) == 0 {
		logger.Errorf("DECODER", "Insufficient audio duration (%.1fs) to decode APT image lines", durationSec)
		return nil, fmt.Errorf("insufficient audio duration to decode APT image")
	}

	logger.Infof("DECODER", "Extracted %d APT image lines (target_width=%d px)", len(rawFloatLines), TargetWidth)

	// Robust float64 contrast normalization to stretch 1st-99th percentile across 0-255 uint8 range
	rawLines := normalizeLines(rawFloatLines)

	// 3. Find global Sync A pulse offset across column sums
	globalOffset := findGlobalSyncOffset(rawLines)
	logger.Infof("DECODER", "Global Sync A pulse offset alignment detected at column x=%d px", globalOffset)

	// 4. Align all lines using global offset shift
	alignedLines := make([][]uint8, len(rawLines))
	for y := 0; y < len(rawLines); y++ {
		alignedLines[y] = shiftLine(rawLines[y], globalOffset)
	}

	// 5. Save full image (Channel A + B side-by-side)
	fullPath := fmt.Sprintf("%s/%s_full.png", outDir, baseName)
	if err := savePNG(fullPath, alignedLines); err != nil {
		logger.Errorf("DECODER", "Failed to save full APT image %s: %v", fullPath, err)
		return nil, fmt.Errorf("failed to save full image: %w", err)
	}

	// 6. Save Channel A (Visible half) and Channel B (IR half) according to NOAA APT specs
	chanALines := extractSubImage(alignedLines, 86, 995)
	chanBLines := extractSubImage(alignedLines, 1126, 2035)

	chanAPath := fmt.Sprintf("%s/%s_chanA.png", outDir, baseName)
	chanBPath := fmt.Sprintf("%s/%s_chanB.png", outDir, baseName)

	_ = savePNG(chanAPath, chanALines)
	_ = savePNG(chanBPath, chanBLines)

	logger.Infof("DECODER", "APT decode completed successfully: lines=%d, sync_offset=%d, full_image=%s", len(alignedLines), globalOffset, fullPath)

	return &DecodeResult{
		FullImagePath: fullPath,
		ChanAPath:     chanAPath,
		ChanBPath:     chanBPath,
		LineCount:     len(alignedLines),
		SyncOffset:    globalOffset,
	}, nil
}

func extractEnvelope(samples []float64, sampleRate int) []float64 {
	sr := float64(sampleRate)
	if sr <= 0 {
		sr = 20800.0
	}
	carrierFreq := 2400.0
	dt := 2.0 * math.Pi * carrierFreq / sr

	env := make([]float64, len(samples))
	iSamples := make([]float64, len(samples))
	qSamples := make([]float64, len(samples))

	// 1. Quadrature mixing with 2400 Hz subcarrier to downconvert 2400 Hz to DC
	for n := 0; n < len(samples); n++ {
		angle := float64(n) * dt
		iSamples[n] = samples[n] * math.Cos(angle)
		qSamples[n] = samples[n] * math.Sin(angle)
	}

	// 2. Low-pass filter (moving average) to reject out-of-band audio noise and 4800Hz harmonics
	// Window size scaled to sample rate (~1 pixel width at TargetWidth=2080, e.g. 5 samples at 20800 Hz => window=2)
	window := int(sr / 8320.0)
	if window < 1 {
		window = 1
	}
	for n := 0; n < len(samples); n++ {
		sumI := 0.0
		sumQ := 0.0
		count := 0
		for w := -window; w <= window; w++ {
			idx := n + w
			if idx >= 0 && idx < len(samples) {
				sumI += iSamples[idx]
				sumQ += qSamples[idx]
				count++
			}
		}
		avgI := sumI / float64(count)
		avgQ := sumQ / float64(count)
		env[n] = math.Sqrt(avgI*avgI + avgQ*avgQ)
	}

	return env
}

func findGlobalSyncOffset(lines [][]uint8) int {
	if len(lines) == 0 {
		return 0
	}

	columnSums := make([]float64, TargetWidth)
	for y := 0; y < len(lines); y++ {
		for x := 0; x < TargetWidth; x++ {
			columnSums[x] += float64(lines[y][x])
		}
	}

	bestX := 0
	maxDiff := -1e9

	// Sync A consists of 7 cycles of 1040 Hz square wave.
	// At 2080 px per 0.5s line, each 1040 Hz cycle is 4 px wide (2 px HIGH, 2 px LOW).
	// Search in the first half of the line (Channel A region) where Sync A is located.
	for x := 0; x < TargetWidth/2; x++ {
		diff := 0.0
		for p := 0; p < 7; p++ {
			h1 := (x + p*4) % TargetWidth
			h2 := (x + p*4 + 1) % TargetWidth
			l1 := (x + p*4 + 2) % TargetWidth
			l2 := (x + p*4 + 3) % TargetWidth
			diff += (columnSums[h1] + columnSums[h2]) - (columnSums[l1] + columnSums[l2])
		}
		if diff > maxDiff {
			maxDiff = diff
			bestX = x
		}
	}

	return bestX
}

func shiftLine(line []uint8, offset int) []uint8 {
	width := len(line)
	shifted := make([]uint8, width)
	for x := 0; x < width; x++ {
		srcX := (x + offset) % width
		shifted[x] = line[srcX]
	}
	return shifted
}

func resampleLine(input []float64, targetLen int) []float64 {
	output := make([]float64, targetLen)
	srcLen := len(input)

	for i := 0; i < targetLen; i++ {
		srcIdx := float64(i) * float64(srcLen) / float64(targetLen)
		idx0 := int(srcIdx)
		idx1 := idx0 + 1
		if idx1 >= srcLen {
			idx1 = idx0
		}
		frac := srcIdx - float64(idx0)
		output[i] = (1.0-frac)*input[idx0] + frac*input[idx1]
	}
	return output
}

func normalizeLines(lines [][]float64) [][]uint8 {
	if len(lines) == 0 {
		return nil
	}

	var allVals []float64
	for y := 0; y < len(lines); y++ {
		allVals = append(allVals, lines[y]...)
	}

	if len(allVals) == 0 {
		return nil
	}

	sort.Float64s(allVals)
	p1Cut := allVals[int(float64(len(allVals))*0.01)]
	p99Cut := allVals[int(float64(len(allVals))*0.99)]

	rng := p99Cut - p1Cut
	if rng <= 1e-9 {
		rng = 1.0
	}

	normalized := make([][]uint8, len(lines))
	for y := 0; y < len(lines); y++ {
		normalized[y] = make([]uint8, len(lines[y]))
		for x := 0; x < len(lines[y]); x++ {
			val := (lines[y][x] - p1Cut) / rng * 255.0
			if val > 255.0 {
				val = 255.0
			}
			if val < 0.0 {
				val = 0.0
			}
			normalized[y][x] = uint8(val)
		}
	}
	return normalized
}

func extractSubImage(lines [][]uint8, startX, endX int) [][]uint8 {
	res := make([][]uint8, len(lines))
	subWidth := endX - startX
	for y := 0; y < len(lines); y++ {
		res[y] = make([]uint8, subWidth)
		for x := 0; x < subWidth; x++ {
			srcX := startX + x
			if srcX < len(lines[y]) {
				res[y][x] = lines[y][srcX]
			}
		}
	}
	return res
}

func savePNG(filename string, lines [][]uint8) error {
	if len(lines) == 0 {
		return fmt.Errorf("empty lines")
	}

	height := len(lines)
	width := len(lines[0])
	img := image.NewGray(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetGray(x, y, color.Gray{Y: lines[y][x]})
		}
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

// DecodeWAVFile loads a WAV audio file and decodes it into APT images
func DecodeWAVFile(wavPath, outDir, baseName string) (*DecodeResult, error) {
	samples, sampleRate, err := dsp.LoadWAV(wavPath)
	if err != nil {
		logger.Errorf("DECODER", "Failed to load WAV file %s: %v", wavPath, err)
		return nil, fmt.Errorf("failed to load WAV file: %w", err)
	}

	return DecodeAPT(samples, sampleRate, outDir, baseName)
}

