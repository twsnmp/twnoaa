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
	FullImagePath string  `json:"full_image_path"`
	ChanAPath     string  `json:"chan_a_path"`
	ChanBPath     string  `json:"chan_b_path"`
	LineCount     int     `json:"line_count"`
	SyncOffset    int     `json:"sync_offset"`
	DecodeMode    string  `json:"decode_mode,omitempty"`
	DayFraction   float64 `json:"day_fraction,omitempty"`
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

	// 1. Extract AM envelope using robust full-wave rectification + low-pass filter
	envelope := extractEnvelope(samples, sampleRate)

	// 2. Line-by-line sync tracking (PLL-like alignment)
	// Nominal samples per line (0.5 sec)
	nominalLineSamples := int(float64(sampleRate) * 0.5)
	if nominalLineSamples <= 0 {
		nominalLineSamples = 10400
	}

	// Sync A pattern: 7 cycles of 1040 Hz square wave
	// Period per cycle in audio samples = sampleRate / 1040 (e.g. 20 samples at 20800 Hz)
	cycleSamples := float64(sampleRate) / 1040.0
	syncLen := int(cycleSamples * 7.0)
	if syncLen < 14 {
		syncLen = 14
	}
	syncPattern := make([]float64, syncLen)
	for c := 0; c < 7; c++ {
		startC := int(float64(c) * cycleSamples)
		midC := int((float64(c) + 0.5) * cycleSamples)
		endC := int(float64(c+1) * cycleSamples)
		if endC > syncLen {
			endC = syncLen
		}
		for k := startC; k < midC && k < syncLen; k++ {
			syncPattern[k] = 1.0
		}
		for k := midC; k < endC && k < syncLen; k++ {
			syncPattern[k] = -1.0
		}
	}

	// Find initial sync in the first 1 line window (nominalLineSamples)
	bestInitPos := 0
	maxInitMetric := -1e9
	searchWindow := nominalLineSamples
	if searchWindow > len(envelope)-syncLen {
		searchWindow = len(envelope) - syncLen
	}
	for p := 0; p < searchWindow; p++ {
		metric := 0.0
		for k := 0; k < syncLen; k++ {
			metric += envelope[p+k] * syncPattern[k]
		}
		if metric > maxInitMetric {
			maxInitMetric = metric
			bestInitPos = p
		}
	}
	if maxInitMetric < 1.0 {
		bestInitPos = 0
	}

	currentPos := bestInitPos
	var rawFloatLines [][]float64
	searchRange := int(cycleSamples * 15) // ±15 cycles search range around expected next line

	for currentPos < len(envelope) {
		endPos := currentPos + nominalLineSamples
		if endPos > len(envelope) {
			// If at least 30% of a line remains, pad and include it as the final line
			if len(envelope)-currentPos < nominalLineSamples/3 {
				break
			}
			padded := make([]float64, nominalLineSamples)
			copy(padded, envelope[currentPos:])
			resampled := resampleLine(padded, TargetWidth)
			rawFloatLines = append(rawFloatLines, resampled)
			break
		}

		lineChunk := envelope[currentPos:endPos]
		resampled := resampleLine(lineChunk, TargetWidth)
		rawFloatLines = append(rawFloatLines, resampled)

		expectedNext := currentPos + nominalLineSamples
		bestNext := expectedNext
		maxMetric := -1e9

		for delta := -searchRange; delta <= searchRange; delta++ {
			p := expectedNext + delta
			if p < 0 || p+syncLen >= len(envelope) {
				continue
			}
			metric := 0.0
			for k := 0; k < syncLen; k++ {
				metric += envelope[p+k] * syncPattern[k]
			}
			if metric > maxMetric {
				maxMetric = metric
				bestNext = p
			}
		}

		if maxMetric > 1.5 {
			currentPos = bestNext
		} else {
			currentPos = expectedNext
		}
	}

	if len(rawFloatLines) == 0 {
		logger.Errorf("DECODER", "Insufficient audio duration (%.1fs) to decode APT image lines", durationSec)
		return nil, fmt.Errorf("insufficient audio duration to decode APT image")
	}

	logger.Infof("DECODER", "Extracted %d APT image lines with line-by-line sync alignment", len(rawFloatLines))

	// Contrast normalization
	alignedLines := normalizeLines(rawFloatLines)

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

	logger.Infof("DECODER", "APT decode completed successfully: lines=%d, full_image=%s", len(alignedLines), fullPath)

	return &DecodeResult{
		FullImagePath: fullPath,
		ChanAPath:     chanAPath,
		ChanBPath:     chanBPath,
		LineCount:     len(alignedLines),
		SyncOffset:    bestInitPos,
		DecodeMode:    "apt",
		DayFraction:   1.0,
	}, nil
}

func extractEnvelope(samples []float64, sampleRate int) []float64 {
	sr := float64(sampleRate)
	if sr <= 0 {
		sr = 20800.0
	}

	// 1. Pre-bandpass filter around 2400 Hz subcarrier (high-pass ~600 Hz + low-pass ~4400 Hz)
	hpAlpha := math.Exp(-2.0 * math.Pi * 600.0 / sr)
	lpAlpha := math.Exp(-2.0 * math.Pi * 4400.0 / sr)
	lpOneMinus := 1.0 - lpAlpha

	// 2. 2nd-order cascaded IIR low-pass filter (~1600 Hz cutoff for AM envelope detection)
	cutoff := 1600.0
	alpha := math.Exp(-2.0 * math.Pi * cutoff / sr)
	oneMinusAlpha := 1.0 - alpha

	env := make([]float64, len(samples))
	hpState := 0.0
	bpState := 0.0
	stage1 := 0.0
	stage2 := 0.0

	for i := 0; i < len(samples); i++ {
		s := samples[i]
		hpState = hpState*hpAlpha + s*(1.0-hpAlpha)
		hpOut := s - hpState
		bpState = bpState*lpAlpha + hpOut*lpOneMinus

		absS := math.Abs(bpState)
		stage1 = stage1*alpha + absS*oneMinusAlpha
		stage2 = stage2*alpha + stage1*oneMinusAlpha
		env[i] = stage2
	}

	return env
}

func shiftLine(line []uint8, offset int) []uint8 {
	width := len(line)
	shifted := make([]uint8, width)
	for x := 0; x < width; x++ {
		srcX := (x + offset) % width
		if srcX < 0 {
			srcX += width
		}
		shifted[x] = line[srcX]
	}
	return shifted
}

func resampleLine(input []float64, targetLen int) []float64 {
	output := make([]float64, targetLen)
	srcLen := len(input)
	if srcLen == 0 {
		return output
	}

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
	p1Cut := allVals[int(float64(len(allVals))*0.02)]
	p99Cut := allVals[int(float64(len(allVals))*0.98)]

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

