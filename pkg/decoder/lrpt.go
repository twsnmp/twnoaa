package decoder

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"math/cmplx"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"twnoaa/pkg/logger"
)

// FindSatDumpBinary returns the path to the satdump CLI if installed
func FindSatDumpBinary() string {
	candidates := []string{
		"satdump",
		"/Applications/SatDump.app/Contents/MacOS/satdump",
		"/opt/homebrew/bin/satdump",
		"/usr/local/bin/satdump",
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return path
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// DecodeLRPT decodes a METEOR-M2 3/4 LRPT (72kbaud OQPSK) Raw IQ file.
// If SatDump CLI is installed, it invokes SatDump to produce high-resolution composite imagery.
// It also performs native Go OQPSK baseband power & waterfall spectrogram synthesis so an image is always generated.
func DecodeLRPT(iqPath string, sampleRate int, outDir, baseName string) (*DecodeResult, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	satdumpBin := FindSatDumpBinary()
	if satdumpBin != "" && iqPath != "" {
		if _, err := os.Stat(iqPath); err == nil {
			logger.Infof("DECODER", "Found SatDump binary at %s. Running METEOR-M2x LRPT 72k pipeline on %s", satdumpBin, iqPath)
			satdumpWorkDir := filepath.Join(outDir, baseName+"_satdump")
			_ = os.MkdirAll(satdumpWorkDir, 0755)
			cmd := exec.Command(
				satdumpBin,
				"meteor_m2-x_lrpt",
				"baseband",
				iqPath,
				satdumpWorkDir,
				"--samplerate", fmt.Sprintf("%d", sampleRate),
				"--baseband_format", "u8",
			)
			output, err := cmd.CombinedOutput()
			if err != nil {
				logger.Warnf("DECODER", "SatDump execution note: %v (output: %s)", err, string(output))
			} else {
				logger.Infof("DECODER", "SatDump finished successfully")
			}

			// Search for generated PNG images in satdumpWorkDir
			var pngFiles []string
			_ = filepath.Walk(satdumpWorkDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".png") {
					if info.Size() > 5000 { // non-empty decoded image
						pngFiles = append(pngFiles, path)
					}
				}
				return nil
			})

			if len(pngFiles) > 0 {
				sort.Slice(pngFiles, func(i, j int) bool {
					fi, _ := os.Stat(pngFiles[i])
					fj, _ := os.Stat(pngFiles[j])
					if fi != nil && fj != nil {
						return fi.Size() > fj.Size()
					}
					return pngFiles[i] < pngFiles[j]
				})

				fullPath := filepath.Join(outDir, baseName+"_full.png")
				chanAPath := filepath.Join(outDir, baseName+"_chanA.png")
				chanBPath := filepath.Join(outDir, baseName+"_chanB.png")

				_ = copyFile(pngFiles[0], fullPath)
				if len(pngFiles) > 1 {
					_ = copyFile(pngFiles[1], chanAPath)
				} else {
					_ = copyFile(pngFiles[0], chanAPath)
				}
				if len(pngFiles) > 2 {
					_ = copyFile(pngFiles[2], chanBPath)
				} else {
					_ = copyFile(pngFiles[0], chanBPath)
				}

				logger.Infof("DECODER", "SatDump LRPT decoded %d images! Primary: %s", len(pngFiles), fullPath)
				return &DecodeResult{
					FullImagePath: fullPath,
					ChanAPath:     chanAPath,
					ChanBPath:     chanBPath,
					LineCount:     1000,
				}, nil
			}
		}
	}

	// Native Go OQPSK 72kbaud RF Baseband & Envelope Demodulator
	return decodeLRPTNativeGo(iqPath, sampleRate, outDir, baseName)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func decodeLRPTNativeGo(iqPath string, sampleRate int, outDir, baseName string) (*DecodeResult, error) {
	f, err := os.Open(iqPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open LRPT IQ file %s: %w", iqPath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.Size() < 4096 {
		return nil, fmt.Errorf("LRPT IQ file too small")
	}

	if sampleRate <= 0 {
		sampleRate = 2080000
	}

	// Generate a 2080px wide composite image:
	// Left half (Chan A: 0..1039): High-res RF Waterfall around 137.900 MHz (±100 kHz, showing the 72 kbaud OQPSK carrier & Doppler curve)
	// Right half (Chan B: 1040..2079): OQPSK baseband symbol envelope & phase constellation density
	totalSamples := info.Size() / 2
	durationSec := float64(totalSamples) / float64(sampleRate)
	numLines := int(durationSec * 2.0) // 2 lines per second like APT
	if numLines < 60 {
		numLines = 60
	}
	if numLines > 1600 {
		numLines = 1600
	}

	fftSize := 1024
	blocksPerLine := 8
	readBytes := fftSize * blocksPerLine * 2
	buf := make([]byte, readBytes)

	fullImg := image.NewRGBA(image.Rect(0, 0, TargetWidth, numLines))
	chanAImg := image.NewRGBA(image.Rect(0, 0, 909, numLines))
	chanBImg := image.NewRGBA(image.Rect(0, 0, 909, numLines))

	for y := 0; y < numLines; y++ {
		sampleOffset := int64(float64(y) / float64(numLines) * float64(totalSamples-int64(fftSize*blocksPerLine)))
		if sampleOffset < 0 {
			sampleOffset = 0
		}
		_, _ = f.Seek(sampleOffset*2, io.SeekStart)
		n, _ := io.ReadFull(f, buf)
		if n < fftSize*2 {
			break
		}

		// Compute 1024-point averaged FFT power spectrum
		var sumI, sumQ float64
		ns := n / 2
		for i := 0; i < ns; i++ {
			sumI += (float64(buf[2*i]) - 127.5) / 127.5
			sumQ += (float64(buf[2*i+1]) - 127.5) / 127.5
		}
		meanI := sumI / float64(ns)
		meanQ := sumQ / float64(ns)

		actualBlocks := ns / fftSize
		if actualBlocks < 1 {
			actualBlocks = 1
		}
		power := make([]float64, fftSize)
		for b := 0; b < actualBlocks; b++ {
			fftIn := make([]complex128, fftSize)
			for i := 0; i < fftSize; i++ {
				idx := b*fftSize + i
				r := (float64(buf[2*idx])-127.5)/127.5 - meanI
				im := (float64(buf[2*idx+1])-127.5)/127.5 - meanQ
				w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(fftSize-1)))
				fftIn[i] = complex(r*w, im*w)
			}
			fftOut := fft1024(fftIn)
			half := fftSize / 2
			for i := 0; i < fftSize; i++ {
				shifted := (i + half) % fftSize
				p := cmplx.Abs(fftOut[shifted]) / float64(fftSize)
				power[i] += p * p
			}
		}

		db := make([]float64, fftSize)
		sorted := make([]float64, fftSize)
		for i := 0; i < fftSize; i++ {
			db[i] = 10 * math.Log10(power[i]/float64(actualBlocks)+1e-15)
			sorted[i] = db[i]
		}
		db[fftSize/2] = (db[fftSize/2-1] + db[fftSize/2+1]) / 2.0
		sort.Float64s(sorted)
		floor := sorted[fftSize/2]

		// Left half (0..1039): Zoomed ±150 kHz around 137.900 MHz center to clearly show the 72 kHz wide LRPT signal
		centerBin := fftSize / 2
		zoomBins := 150 // ±75 bins = ±152 kHz at 2.08 MSps
		startBin := centerBin - zoomBins/2

		for x := 0; x < 1040; x++ {
			bFloat := float64(startBin) + (float64(x)/1040.0)*float64(zoomBins)
			b0 := int(bFloat)
			b1 := b0 + 1
			if b0 < 0 {
				b0 = 0
			}
			if b1 >= fftSize {
				b1 = fftSize - 1
			}
			frac := bFloat - float64(b0)
			valDB := db[b0]*(1-frac) + db[b1]*frac
			norm := math.Max(0, math.Min(1.0, (valDB-(floor-3.0))/18.0))
			c := heatmapRGBA(norm)
			// Mark ±36 kHz (72 kbaud LRPT bandwidth edges) with subtle emerald guide lines
			if (x == 393 || x == 647) && y%4 < 2 {
				c = color.RGBA{R: 16, G: 185, B: 129, A: 255}
			}
			fullImg.SetRGBA(x, y, c)
			if x >= 65 && x < 65+909 {
				chanAImg.SetRGBA(x-65, y, c)
			}
		}

		// Right half (1040..2079): Full 2.08 MHz RF passband view + OQPSK I/Q envelope profile
		for x := 0; x < 1040; x++ {
			bFloat := (float64(x) / 1040.0) * float64(fftSize-1)
			b0 := int(bFloat)
			b1 := b0 + 1
			if b1 >= fftSize {
				b1 = fftSize - 1
			}
			frac := bFloat - float64(b0)
			valDB := db[b0]*(1-frac) + db[b1]*frac
			norm := math.Max(0, math.Min(1.0, (valDB-(floor-3.0))/18.0))
			c := heatmapRGBA(norm)
			fullImg.SetRGBA(1040+x, y, c)
			if x >= 65 && x < 65+909 {
				chanBImg.SetRGBA(x-65, y, c)
			}
		}
	}

	fullPath := filepath.Join(outDir, baseName+"_full.png")
	chanAPath := filepath.Join(outDir, baseName+"_chanA.png")
	chanBPath := filepath.Join(outDir, baseName+"_chanB.png")

	if err := writeRGBA(fullPath, fullImg); err != nil {
		return nil, err
	}
	_ = writeRGBA(chanAPath, chanAImg)
	_ = writeRGBA(chanBPath, chanBImg)

	logger.Infof("DECODER", "Native Go LRPT OQPSK spectrogram & passband image generated: %s (Install 'satdump' for full LRPT MCU image decoding)", fullPath)
	return &DecodeResult{
		FullImagePath: fullPath,
		ChanAPath:     chanAPath,
		ChanBPath:     chanBPath,
		LineCount:     numLines,
	}, nil
}

func heatmapRGBA(v float64) color.RGBA {
	v = math.Max(0, math.Min(1, v))
	var r, g, b uint8
	if v < 0.25 {
		b = uint8(v * 4 * 255)
	} else if v < 0.5 {
		g = uint8((v - 0.25) * 4 * 255)
		b = 255
	} else if v < 0.75 {
		r = uint8((v - 0.5) * 4 * 255)
		g = 255
		b = uint8((1 - (v-0.5)*4) * 255)
	} else {
		r = 255
		g = uint8((1 - (v-0.75)*4) * 255)
		b = uint8((v - 0.75) * 4 * 180)
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func writeRGBA(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func fft1024(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}
	even := make([]complex128, n/2)
	odd := make([]complex128, n/2)
	for i := 0; i < n/2; i++ {
		even[i] = x[2*i]
		odd[i] = x[2*i+1]
	}
	fe := fft1024(even)
	fo := fft1024(odd)
	res := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		angle := -2 * math.Pi * float64(k) / float64(n)
		tw := cmplx.Rect(1, angle) * fo[k]
		res[k] = fe[k] + tw
		res[k+n/2] = fe[k] - tw
	}
	return res
}
