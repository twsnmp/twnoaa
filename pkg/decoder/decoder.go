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

// DecodeWAVFile loads a WAV file and runs DecodeAPT
func DecodeWAVFile(wavPath, outDir, baseName string) (*DecodeResult, error) {
	samples, sampleRate, err := dsp.LoadWAV(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load WAV file %s: %w", wavPath, err)
	}
	return DecodeAPT(samples, sampleRate, outDir, baseName)
}

// DecodeAPT processes demodulated audio float64 samples (-1.0 to 1.0) and generates APT satellite images
func DecodeAPT(samples []float64, sampleRate int, outDir, baseName string) (*DecodeResult, error) {
	if len(samples) == 0 {
		logger.Errorf("DECODER", "DecodeAPT failed: no audio samples provided")
		return nil, fmt.Errorf("no audio samples provided")
	}

	durationSec := float64(len(samples)) / float64(sampleRate)
	logger.Infof("DECODER", "Starting APT decode: baseName=%s, total_samples=%d (duration=%.1fs), sample_rate=%dHz", baseName, len(samples), durationSec, sampleRate)

	if err := os.MkdirAll(outDir, 0755); err != nil {
		logger.Errorf("DECODER", "Failed to create output directory %s: %v", outDir, err)
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	// 1. Extract 2400 Hz AM envelope with 1600 Hz LPF
	envelope := extractEnvelope(samples, sampleRate)

	// 2. Slice into 0.5s line chunks and resample to 2080 px TargetWidth
	samplesPerLine := int(float64(sampleRate) * 0.5)
	if samplesPerLine <= 0 {
		samplesPerLine = 10400
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

	// Contrast normalization to stretch 1st-99th percentile across 0-255 uint8 range
	rawLines := normalizeLines(rawFloatLines)

	// 3. Global Sync A pulse offset detection
	globalOffset := findGlobalSyncOffset(rawLines)
	logger.Infof("DECODER", "Global Sync A pulse offset alignment detected at column x=%d px", globalOffset)

	// 4. Per-line Sync A fine alignment (Line Sync Matching to eliminate line jitter/skew)
	alignedLines := make([][]uint8, len(rawLines))
	for y := 0; y < len(rawLines); y++ {
		lineOffset := findLineSyncOffset(rawLines[y], globalOffset)
		alignedLines[y] = shiftLine(rawLines[y], lineOffset)
	}

	// 5. Save full image
	fullPath := fmt.Sprintf("%s/%s_full.png", outDir, baseName)
	if err := savePNG(fullPath, alignedLines); err != nil {
		logger.Errorf("DECODER", "Failed to save full APT image %s: %v", fullPath, err)
		return nil, fmt.Errorf("failed to save full image: %w", err)
	}

	// 6. Save Channel A (Visible) and Channel B (IR)
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

	envRaw := make([]float64, len(samples))

	// 1. Quadrature mixing with 2400 Hz subcarrier to downconvert 2400 Hz to DC
	for n := 0; n < len(samples); n++ {
		angle := float64(n) * dt
		iVal := samples[n] * math.Cos(angle)
		qVal := samples[n] * math.Sin(angle)
		envRaw[n] = math.Sqrt(iVal*iVal + qVal*qVal)
	}

	// 2. Baseband Low-Pass Filter (~1600 Hz cutoff) to remove pixel noise and 4800Hz subcarrier ripple
	alpha := math.Exp(-2.0 * math.Pi * 1600.0 / sr)
	oneMinusAlpha := 1.0 - alpha

	envFiltered := make([]float64, len(samples))
	var lpfState float64
	for n := 0; n < len(samples); n++ {
		lpfState = lpfState*alpha + envRaw[n]*oneMinusAlpha
		envFiltered[n] = lpfState
	}

	return envFiltered
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

func findLineSyncOffset(line []uint8, globalOffset int) int {
	searchRange := 12
	bestOffset := globalOffset
	maxDiff := -1e9

	for delta := -searchRange; delta <= searchRange; delta++ {
		x := (globalOffset + delta + TargetWidth) % TargetWidth
		diff := 0.0
		for p := 0; p < 7; p++ {
			h1 := (x + p*4) % TargetWidth
			h2 := (x + p*4 + 1) % TargetWidth
			l1 := (x + p*4 + 2) % TargetWidth
			l2 := (x + p*4 + 3) % TargetWidth
			diff += float64(line[h1]) + float64(line[h2]) - (float64(line[l1]) + float64(line[l2]))
		}
		if diff > maxDiff {
			maxDiff = diff
			bestOffset = x
		}
	}

	return bestOffset
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
	if len(lines) == 0 {
		return nil
	}
	subWidth := endX - startX + 1
	if subWidth <= 0 {
		return nil
	}

	sub := make([][]uint8, len(lines))
	for y := 0; y < len(lines); y++ {
		sub[y] = make([]uint8, subWidth)
		for x := 0; x < subWidth; x++ {
			srcX := startX + x
			if srcX >= 0 && srcX < len(lines[y]) {
				sub[y][x] = lines[y][srcX]
			}
		}
	}
	return sub
}

func savePNG(filePath string, lines [][]uint8) error {
	if len(lines) == 0 || len(lines[0]) == 0 {
		return fmt.Errorf("empty image lines")
	}

	height := len(lines)
	width := len(lines[0])
	img := image.NewGray(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetGray(x, y, color.Gray{Y: lines[y][x]})
		}
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}
