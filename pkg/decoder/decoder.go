package decoder

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
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
		return nil, fmt.Errorf("no audio samples provided")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	// 1. Extract AM envelope
	envelope := extractEnvelope(samples)

	// 2. Slice into 0.5s line chunks and resample to 2080 px TargetWidth
	samplesPerLine := int(float64(sampleRate) * 0.5)
	if samplesPerLine <= 0 {
		samplesPerLine = 10400 // fallback for 20800 Hz
	}

	var rawLines [][]uint8
	for i := 0; i+samplesPerLine <= len(envelope); i += samplesPerLine {
		line := resampleLine(envelope[i:i+samplesPerLine], TargetWidth)
		rawLines = append(rawLines, line)
	}

	if len(rawLines) == 0 {
		return nil, fmt.Errorf("insufficient audio duration to decode APT image")
	}

	// Dynamic contrast normalization to avoid white-saturation when RF signal power is strong
	rawLines = normalizeLines(rawLines)

	// 3. Find global Sync A pulse offset across column sums
	globalOffset := findGlobalSyncOffset(rawLines)

	// 4. Align all lines using global offset shift
	alignedLines := make([][]uint8, len(rawLines))
	for y := 0; y < len(rawLines); y++ {
		alignedLines[y] = shiftLine(rawLines[y], globalOffset)
	}

	// 5. Save full image (Channel A + B side-by-side)
	fullPath := fmt.Sprintf("%s/%s_full.png", outDir, baseName)
	if err := savePNG(fullPath, alignedLines); err != nil {
		return nil, fmt.Errorf("failed to save full image: %w", err)
	}

	// 6. Save Channel A (Visible half) and Channel B (IR half)
	chanALines := extractSubImage(alignedLines, 80, 990)
	chanBLines := extractSubImage(alignedLines, 1120, 2030)

	chanAPath := fmt.Sprintf("%s/%s_chanA.png", outDir, baseName)
	chanBPath := fmt.Sprintf("%s/%s_chanB.png", outDir, baseName)

	_ = savePNG(chanAPath, chanALines)
	_ = savePNG(chanBPath, chanBLines)

	return &DecodeResult{
		FullImagePath: fullPath,
		ChanAPath:     chanAPath,
		ChanBPath:     chanBPath,
		LineCount:     len(alignedLines),
		SyncOffset:    globalOffset,
	}, nil
}

func extractEnvelope(samples []float64) []float64 {
	env := make([]float64, len(samples))
	window := 5
	for i := 0; i < len(samples); i++ {
		sum := 0.0
		count := 0
		for w := -window; w <= window; w++ {
			idx := i + w
			if idx >= 0 && idx < len(samples) {
				sum += math.Abs(samples[idx])
				count++
			}
		}
		env[i] = sum / float64(count)
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

	for x := 0; x < TargetWidth/2; x++ {
		diff := 0.0
		for p := 0; p < 7; p++ {
			p1 := (x + p*12) % TargetWidth
			p2 := (x + p*12 + 6) % TargetWidth
			diff += columnSums[p1] - columnSums[p2]
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

func resampleLine(input []float64, targetLen int) []uint8 {
	output := make([]uint8, targetLen)
	srcLen := len(input)

	for i := 0; i < targetLen; i++ {
		srcIdx := float64(i) * float64(srcLen) / float64(targetLen)
		idx0 := int(srcIdx)
		idx1 := idx0 + 1
		if idx1 >= srcLen {
			idx1 = idx0
		}
		frac := srcIdx - float64(idx0)

		val := (1.0-frac)*input[idx0] + frac*input[idx1]
		pixelVal := val * 2.0 * 255.0
		if pixelVal > 255 {
			pixelVal = 255
		}
		if pixelVal < 0 {
			pixelVal = 0
		}

		output[i] = uint8(pixelVal)
	}
	return output
}

func normalizeLines(lines [][]uint8) [][]uint8 {
	if len(lines) == 0 {
		return lines
	}
	minV := uint8(255)
	maxV := uint8(0)

	for y := 0; y < len(lines); y++ {
		for x := 0; x < len(lines[y]); x++ {
			v := lines[y][x]
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
	}

	if maxV <= minV || (maxV-minV) < 10 {
		return lines
	}

	minF := float64(minV)
	rngF := float64(maxV - minV)
	normalized := make([][]uint8, len(lines))

	for y := 0; y < len(lines); y++ {
		normalized[y] = make([]uint8, len(lines[y]))
		for x := 0; x < len(lines[y]); x++ {
			v := float64(lines[y][x]) - minF
			val := (v / rngF) * 255.0
			if val > 255 {
				val = 255
			}
			if val < 0 {
				val = 0
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
