package decoder

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/cmplx"
	"os"
	"path/filepath"

	"twnoaa/pkg/dsp"
	"twnoaa/pkg/logger"
)

// SSTVMode describes the timing and resolution of an analog SSTV mode
type SSTVMode struct {
	Name       string
	VISCode    int
	Width      int
	Height     int
	LinePeriod float64 // Total duration of one frame unit in seconds (2 lines for PD120/Robot36, 1 line for Martin M1)
	SyncMs     float64
	PorchMs    float64
}

var (
	ModePD120 = SSTVMode{
		Name:       "PD120",
		VISCode:    95,
		Width:      640,
		Height:     496,
		LinePeriod: 0.50848, // 20ms sync + 2.08ms porch + 4 * 121.6ms (Y0, R-Y, B-Y, Y1) = 508.48ms per 2 lines
		SyncMs:     20.0,
		PorchMs:    2.08,
	}
	ModeRobot36 = SSTVMode{
		Name:       "Robot 36",
		VISCode:    8,
		Width:      320,
		Height:     240,
		LinePeriod: 0.150, // 9ms sync + 3ms porch + 88ms Y + 4.5ms sep + 1.5ms porch + 44ms R-Y/B-Y = 150ms per line
		SyncMs:     9.0,
		PorchMs:    3.0,
	}
	ModeMartinM1 = SSTVMode{
		Name:       "Martin M1",
		VISCode:    44,
		Width:      320,
		Height:     256,
		LinePeriod: 0.446446, // 4.862ms sync + 0.572ms porch + 3*(146.432ms + 0.572ms) = 446.446ms per line
		SyncMs:     4.862,
		PorchMs:    0.572,
	}
)

// DecodeSSTV decodes an analog SSTV audio signal (1100Hz - 2300Hz FM subcarrier) into full-color PNG images
// using 100% pure Go DSP (analytic signal via FFT Hilbert transform + instantaneous frequency discriminator + sync lock).
func DecodeSSTV(samples []float64, sampleRate int, outDir, baseName string) (*DecodeResult, error) {
	if len(samples) < sampleRate {
		return nil, fmt.Errorf("insufficient audio samples for SSTV decode")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	logger.Infof("DECODER", "Starting pure-Go SSTV color decode: baseName=%s, samples=%d (%.1fs), sampleRate=%dHz",
		baseName, len(samples), float64(len(samples))/float64(sampleRate), sampleRate)

	// 1. Extract instantaneous frequency f(t) in Hz for each sample using Analytic Signal (Hilbert Transform)
	instFreq := computeInstantaneousFreq(samples, sampleRate)

	// 2. Auto-detect SSTV Mode (PD120, Robot 36, or Martin M1) via sync pulse periodicity (1200 Hz pulses)
	mode, syncStartSample := detectSSTVModeAndSync(instFreq, sampleRate)
	logger.Infof("DECODER", "Detected SSTV mode: %s (sync start sample: %d)", mode.Name, syncStartSample)

	// 3. Decode lines according to detected mode
	var rgbImg *image.RGBA
	var lumImg *image.Gray
	var chromaImg *image.RGBA

	switch mode.Name {
	case "Robot 36":
		rgbImg, lumImg, chromaImg = decodeRobot36(instFreq, sampleRate, syncStartSample)
	case "Martin M1":
		rgbImg, lumImg, chromaImg = decodeMartinM1(instFreq, sampleRate, syncStartSample)
	default:
		rgbImg, lumImg, chromaImg = decodePD120(instFreq, sampleRate, syncStartSample)
	}

	fullPath := filepath.Join(outDir, baseName+"_full.png")
	chanAPath := filepath.Join(outDir, baseName+"_chanA.png")
	chanBPath := filepath.Join(outDir, baseName+"_chanB.png")

	if err := saveImagePNG(fullPath, rgbImg); err != nil {
		return nil, fmt.Errorf("failed to save SSTV full color image: %w", err)
	}
	_ = saveImagePNG(chanAPath, lumImg)
	_ = saveImagePNG(chanBPath, chromaImg)

	logger.Infof("DECODER", "Pure-Go SSTV decode completed: mode=%s, size=%dx%d, image=%s",
		mode.Name, rgbImg.Bounds().Dx(), rgbImg.Bounds().Dy(), fullPath)

	return &DecodeResult{
		FullImagePath: fullPath,
		ChanAPath:     chanAPath,
		ChanBPath:     chanBPath,
		LineCount:     rgbImg.Bounds().Dy(),
		SyncOffset:    syncStartSample,
		DecodeMode:    "sstv",
		DayFraction:   1.0,
	}, nil
}

// computeInstantaneousFreq computes the instantaneous audio frequency (Hz) at each sample
// using overlapped FFT-based Hilbert Analytic Signal + phase differentiation + low-pass smoothing.
func computeInstantaneousFreq(samples []float64, sampleRate int) []float64 {
	n := len(samples)
	sr := float64(sampleRate)
	freqs := make([]float64, n)

	// Process in overlapped blocks of 4096 samples (2048 valid center samples)
	blockSize := 4096
	validStart := 1024
	validEnd := 3072
	step := validEnd - validStart // 2048

	// Frequency bin indices for bandpass 1000 Hz .. 2500 Hz
	minBin := int(950.0 * float64(blockSize) / sr)
	maxBin := int(2550.0 * float64(blockSize) / sr)
	if minBin < 1 {
		minBin = 1
	}
	if maxBin >= blockSize/2 {
		maxBin = blockSize/2 - 1
	}

	rawFreq := make([]float64, n)
	for pos := 0; pos < n; pos += step {
		fftBuf := make([]complex128, blockSize)
		for i := 0; i < blockSize; i++ {
			srcIdx := pos + i - validStart
			if srcIdx >= 0 && srcIdx < n {
				fftBuf[i] = complex(samples[srcIdx], 0)
			}
		}

		spec := fftRadix2(fftBuf)

		// Analytic signal filter: zero negative frequencies, double positive frequencies inside [minBin..maxBin]
		for k := 0; k < blockSize; k++ {
			if k >= minBin && k <= maxBin {
				spec[k] *= 2.0
			} else {
				spec[k] = 0
			}
		}

		// Inverse FFT via conjugate FFT
		for k := range spec {
			spec[k] = cmplx.Conj(spec[k])
		}
		analytic := fftRadix2(spec)
		invScale := 1.0 / float64(blockSize)

		for i := validStart; i < validEnd; i++ {
			dstIdx := pos + (i - validStart)
			if dstIdx <= 0 || dstIdx >= n {
				continue
			}
			z0 := cmplx.Conj(analytic[i-1]) * complex(invScale, 0)
			z1 := cmplx.Conj(analytic[i]) * complex(invScale, 0)
			// Phase difference between adjacent samples
			prod := z1 * cmplx.Conj(z0)
			dPhase := math.Atan2(imag(prod), real(prod))
			if dPhase < 0 {
				dPhase += 2 * math.Pi
			}
			fHz := dPhase * sr / (2.0 * math.Pi)
			if fHz < 1000.0 {
				fHz = 1000.0
			}
			if fHz > 2500.0 {
				fHz = 2500.0
			}
			rawFreq[dstIdx] = fHz
		}
	}

	// Smooth instantaneous frequency with a 2-stage low-pass filter (~2.8 kHz cutoff)
	alpha := math.Exp(-2.0 * math.Pi * 2800.0 / sr)
	oneMinus := 1.0 - alpha
	s1 := 1500.0
	s2 := 1500.0
	for i := 0; i < n; i++ {
		f := rawFreq[i]
		if f == 0 {
			f = 1500.0
		}
		s1 = s1*alpha + f*oneMinus
		s2 = s2*alpha + s1*oneMinus
		freqs[i] = s2
	}

	return freqs
}

// detectSSTVModeAndSync correlates the 1200 Hz sync pulse train against candidate SSTV modes
func detectSSTVModeAndSync(instFreq []float64, sampleRate int) (SSTVMode, int) {
	sr := float64(sampleRate)
	// Build a sync-indicator signal: +1 when freq is in [1120..1280 Hz] (1200Hz sync), -0.2 otherwise
	syncSig := make([]float64, len(instFreq))
	for i, f := range instFreq {
		if f >= 1120.0 && f <= 1285.0 {
			syncSig[i] = 1.0
		} else {
			syncSig[i] = -0.25
		}
	}

	candidates := []SSTVMode{ModePD120, ModeRobot36, ModeMartinM1}
	bestMode := ModePD120
	bestScore := -1e9
	bestOffset := 0

	for _, m := range candidates {
		periodSamples := int(math.Round(m.LinePeriod * sr))
		syncSamples := int(math.Round((m.SyncMs / 1000.0) * sr))
		if periodSamples <= 0 || len(syncSig) < periodSamples*3 {
			continue
		}

		maxLinesToTest := len(syncSig) / periodSamples
		if maxLinesToTest > 20 {
			maxLinesToTest = 20
		}

		// Search offset across 1 full line period in coarse steps of 10 samples
		step := 10
		for offset := 0; offset < periodSamples && offset+maxLinesToTest*periodSamples < len(syncSig); offset += step {
			score := 0.0
			for line := 0; line < maxLinesToTest; line++ {
				base := offset + line*periodSamples
				lineSum := 0.0
				for k := 0; k < syncSamples && base+k < len(syncSig); k++ {
					lineSum += syncSig[base+k]
				}
				score += lineSum / float64(syncSamples)
			}
			score /= float64(maxLinesToTest)
			if score > bestScore {
				bestScore = score
				bestMode = m
				bestOffset = offset
			}
		}
	}

	// Fine-tune bestOffset within ±15 samples
	periodSamples := int(math.Round(bestMode.LinePeriod * sr))
	syncSamples := int(math.Round((bestMode.SyncMs / 1000.0) * sr))
	fineBest := bestOffset
	fineMax := -1e9
	for delta := -15; delta <= 15; delta++ {
		off := bestOffset + delta
		if off < 0 || off+periodSamples >= len(syncSig) {
			continue
		}
		s := 0.0
		count := 0
		for line := 0; line < 15 && off+line*periodSamples+syncSamples < len(syncSig); line++ {
			base := off + line*periodSamples
			for k := 0; k < syncSamples; k++ {
				s += syncSig[base+k]
			}
			count += syncSamples
		}
		if count > 0 && s/float64(count) > fineMax {
			fineMax = s / float64(count)
			fineBest = off
		}
	}

	return bestMode, fineBest
}

// freqToLevel maps [1500 Hz .. 2300 Hz] -> [0.0 .. 255.0]
func freqToLevel(f float64) float64 {
	v := (f - 1500.0) / 800.0 * 255.0
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func ycrcbToRGB(y, cr, cb float64) color.RGBA {
	r := y + 1.402*(cr-128.0)
	g := y - 0.344136*(cb-128.0) - 0.714136*(cr-128.0)
	b := y + 1.772*(cb-128.0)
	return color.RGBA{
		R: uint8(math.Max(0, math.Min(255, r))),
		G: uint8(math.Max(0, math.Min(255, g))),
		B: uint8(math.Max(0, math.Min(255, b))),
		A: 255,
	}
}

// decodePD120 decodes PD120 (640x496, 2 lines per 508.48ms frame: Sync 20ms, Porch 2.08ms, Y0 121.6ms, R-Y 121.6ms, B-Y 121.6ms, Y1 121.6ms)
func decodePD120(instFreq []float64, sampleRate int, startSample int) (*image.RGBA, *image.Gray, *image.RGBA) {
	sr := float64(sampleRate)
	width := 640
	frameSamples := 0.50848 * sr
	syncSamples := int(0.020 * sr)
	searchRange := int(0.008 * sr) // ±8ms line-by-line sync tracking

	numFrames := int(float64(len(instFreq)-startSample) / frameSamples)
	if numFrames < 1 {
		numFrames = 1
	}
	if numFrames > 248 {
		numFrames = 248
	}
	height := numFrames * 2

	rgbImg := image.NewRGBA(image.Rect(0, 0, width, height))
	lumImg := image.NewGray(image.Rect(0, 0, width, height))
	chromaImg := image.NewRGBA(image.Rect(0, 0, width, height))

	currStart := float64(startSample)
	for f := 0; f < numFrames; f++ {
		baseIdx := int(math.Round(currStart))
		if baseIdx+int(frameSamples) > len(instFreq) {
			break
		}

		// Line-by-line sync tracking around expected start
		bestPos := baseIdx
		bestSyncMetric := -1e9
		for d := -searchRange; d <= searchRange; d++ {
			p := baseIdx + d
			if p < 0 || p+syncSamples >= len(instFreq) {
				continue
			}
			m := 0.0
			for k := 0; k < syncSamples; k++ {
				diff := math.Abs(instFreq[p+k] - 1200.0)
				if diff < 85.0 {
					m += (85.0 - diff)
				}
			}
			if m > bestSyncMetric {
				bestSyncMetric = m
				bestPos = p
			}
		}
		if bestSyncMetric > float64(syncSamples)*20.0 {
			currStart = float64(bestPos)
		}

		// Offsets in seconds from start of sync pulse
		y0Start := currStart + (0.020+0.00208)*sr
		ryStart := y0Start + 0.1216*sr
		byStart := ryStart + 0.1216*sr
		y1Start := byStart + 0.1216*sr
		segSamples := 0.1216 * sr

		yRow0 := f * 2
		yRow1 := f*2 + 1

		for x := 0; x < width; x++ {
			ratio := (float64(x) + 0.5) / float64(width)
			idxY0 := clampIdx(int(y0Start+ratio*segSamples), len(instFreq))
			idxRY := clampIdx(int(ryStart+ratio*segSamples), len(instFreq))
			idxBY := clampIdx(int(byStart+ratio*segSamples), len(instFreq))
			idxY1 := clampIdx(int(y1Start+ratio*segSamples), len(instFreq))

			y0 := freqToLevel(instFreq[idxY0])
			cr := freqToLevel(instFreq[idxRY])
			cb := freqToLevel(instFreq[idxBY])
			y1 := freqToLevel(instFreq[idxY1])

			c0 := ycrcbToRGB(y0, cr, cb)
			c1 := ycrcbToRGB(y1, cr, cb)
			rgbImg.SetRGBA(x, yRow0, c0)
			rgbImg.SetRGBA(x, yRow1, c1)

			lumImg.SetGray(x, yRow0, color.Gray{Y: uint8(y0)})
			lumImg.SetGray(x, yRow1, color.Gray{Y: uint8(y1)})

			chColor := ycrcbToRGB(140, cr, cb)
			chromaImg.SetRGBA(x, yRow0, chColor)
			chromaImg.SetRGBA(x, yRow1, chColor)
		}

		currStart += frameSamples
	}

	return rgbImg, lumImg, chromaImg
}

// decodeRobot36 decodes Robot 36 (320x240, 150ms per line)
func decodeRobot36(instFreq []float64, sampleRate int, startSample int) (*image.RGBA, *image.Gray, *image.RGBA) {
	sr := float64(sampleRate)
	width := 320
	lineSamples := 0.150 * sr
	numLines := int(float64(len(instFreq)-startSample) / lineSamples)
	if numLines < 1 {
		numLines = 1
	}
	if numLines > 240 {
		numLines = 240
	}

	rgbImg := image.NewRGBA(image.Rect(0, 0, width, numLines))
	lumImg := image.NewGray(image.Rect(0, 0, width, numLines))
	chromaImg := image.NewRGBA(image.Rect(0, 0, width, numLines))

	prevCr := make([]float64, width)
	prevCb := make([]float64, width)
	for i := range prevCr {
		prevCr[i] = 128.0
		prevCb[i] = 128.0
	}

	for y := 0; y < numLines; y++ {
		lineStart := float64(startSample) + float64(y)*lineSamples
		yStart := lineStart + 0.012*sr
		yLen := 0.088 * sr
		cStart := lineStart + 0.106*sr
		cLen := 0.044 * sr

		isEven := (y % 2) == 0
		for x := 0; x < width; x++ {
			ratio := (float64(x) + 0.5) / float64(width)
			idxY := clampIdx(int(yStart+ratio*yLen), len(instFreq))
			idxC := clampIdx(int(cStart+ratio*cLen), len(instFreq))

			lum := freqToLevel(instFreq[idxY])
			cVal := freqToLevel(instFreq[idxC])
			if isEven {
				prevCr[x] = cVal
			} else {
				prevCb[x] = cVal
			}

			c := ycrcbToRGB(lum, prevCr[x], prevCb[x])
			rgbImg.SetRGBA(x, y, c)
			lumImg.SetGray(x, y, color.Gray{Y: uint8(lum)})
			chromaImg.SetRGBA(x, y, ycrcbToRGB(140, prevCr[x], prevCb[x]))
		}
	}
	return rgbImg, lumImg, chromaImg
}

// decodeMartinM1 decodes Martin M1 (320x256, G-B-R sequential scan per line)
func decodeMartinM1(instFreq []float64, sampleRate int, startSample int) (*image.RGBA, *image.Gray, *image.RGBA) {
	sr := float64(sampleRate)
	width := 320
	lineSamples := 0.446446 * sr
	numLines := int(float64(len(instFreq)-startSample) / lineSamples)
	if numLines < 1 {
		numLines = 1
	}
	if numLines > 256 {
		numLines = 256
	}

	rgbImg := image.NewRGBA(image.Rect(0, 0, width, numLines))
	lumImg := image.NewGray(image.Rect(0, 0, width, numLines))
	chromaImg := image.NewRGBA(image.Rect(0, 0, width, numLines))

	for y := 0; y < numLines; y++ {
		lineStart := float64(startSample) + float64(y)*lineSamples
		gStart := lineStart + (0.004862+0.000572)*sr
		chanLen := 0.146432 * sr
		bStart := gStart + chanLen + 0.000572*sr
		rStart := bStart + chanLen + 0.000572*sr

		for x := 0; x < width; x++ {
			ratio := (float64(x) + 0.5) / float64(width)
			idxG := clampIdx(int(gStart+ratio*chanLen), len(instFreq))
			idxB := clampIdx(int(bStart+ratio*chanLen), len(instFreq))
			idxR := clampIdx(int(rStart+ratio*chanLen), len(instFreq))

			g := uint8(freqToLevel(instFreq[idxG]))
			b := uint8(freqToLevel(instFreq[idxB]))
			r := uint8(freqToLevel(instFreq[idxR]))

			rgbImg.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			lum := uint8(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b))
			lumImg.SetGray(x, y, color.Gray{Y: lum})
			chromaImg.SetRGBA(x, y, color.RGBA{R: r, G: 0, B: b, A: 255})
		}
	}
	return rgbImg, lumImg, chromaImg
}

func clampIdx(idx, maxLen int) int {
	if idx < 0 {
		return 0
	}
	if idx >= maxLen {
		return maxLen - 1
	}
	return idx
}

func saveImagePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// DecodeSSTVWAVFile loads a WAV audio file and decodes it as SSTV
func DecodeSSTVWAVFile(wavPath, outDir, baseName string) (*DecodeResult, error) {
	samples, sampleRate, err := dsp.LoadWAV(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSTV WAV file: %w", err)
	}
	return DecodeSSTV(samples, sampleRate, outDir, baseName)
}
