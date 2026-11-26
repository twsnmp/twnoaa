package dsp

import (
	"math"
	"sort"
)

type biquadFilter struct {
	b0, b1, b2 float64
	a1, a2     float64
	z1, z2     float64
}

func newBiquadHighpass(sampleRate int, cutoffHz, q float64) *biquadFilter {
	w0 := 2.0 * math.Pi * cutoffHz / float64(sampleRate)
	cosW0 := math.Cos(w0)
	alpha := math.Sin(w0) / (2.0 * q)

	a0 := 1.0 + alpha
	return &biquadFilter{
		b0: ((1.0 + cosW0) / 2.0) / a0,
		b1: (-(1.0 + cosW0)) / a0,
		b2: ((1.0 + cosW0) / 2.0) / a0,
		a1: (-2.0 * cosW0) / a0,
		a2: (1.0 - alpha) / a0,
	}
}

func newBiquadLowpass(sampleRate int, cutoffHz, q float64) *biquadFilter {
	w0 := 2.0 * math.Pi * cutoffHz / float64(sampleRate)
	cosW0 := math.Cos(w0)
	alpha := math.Sin(w0) / (2.0 * q)

	a0 := 1.0 + alpha
	return &biquadFilter{
		b0: ((1.0 - cosW0) / 2.0) / a0,
		b1: (1.0 - cosW0) / a0,
		b2: ((1.0 - cosW0) / 2.0) / a0,
		a1: (-2.0 * cosW0) / a0,
		a2: (1.0 - alpha) / a0,
	}
}

func (f *biquadFilter) process(x float64) float64 {
	y := f.b0*x + f.z1
	f.z1 = f.b1*x - f.a1*y + f.z2
	f.z2 = f.b2*x - f.a2*y
	return y
}

// ExtractVoiceSegments detects FM quieting / voice-active segments in demodulated FM audio
// (where idle squelch-open static has high-frequency white noise and high RMS, while carrier/voice
// drops high-frequency noise and concentrates energy in the speech band).
// Returns the concatenated active segments (with padding and smooth fades) and whether
// selective extraction occurred. If the audio is too short, entirely active, or no distinct
// segments are found, it returns the original samples with extracted=false.
func ExtractVoiceSegments(samples []float64, sampleRate int) ([]float64, bool) {
	if sampleRate <= 0 || len(samples) < sampleRate*2 {
		return samples, false
	}

	winSize := sampleRate / 2 // 0.5-second analysis windows
	if winSize < 64 {
		winSize = 64
	}
	numFrames := len(samples) / winSize
	if numFrames < 4 {
		return samples, false
	}

	rmsVals := make([]float64, numFrames)
	hfVals := make([]float64, numFrames)

	for i := 0; i < numFrames; i++ {
		start := i * winSize
		end := start + winSize
		var sumSq, diffSq float64
		for j := start; j < end; j++ {
			s := samples[j]
			sumSq += s * s
			if j > start {
				d := s - samples[j-1]
				diffSq += d * d
			}
		}
		rms := math.Sqrt(sumSq / float64(winSize))
		diffRMS := math.Sqrt(diffSq / float64(winSize-1))
		rmsVals[i] = rms
		if rms > 1e-9 {
			hfVals[i] = diffRMS / rms
		}
	}

	medRMS := medianFloat64(rmsVals)
	medHF := medianFloat64(hfVals)

	// If median HF ratio is already low (<= 0.55), the recording is mostly carrier/voice rather than static.
	if medHF <= 0.55 || medRMS <= 1e-9 {
		return samples, false
	}

	active := make([]bool, numFrames)
	activeCount := 0
	for i := 0; i < numFrames; i++ {
		if hfVals[i] < medHF*0.80 || rmsVals[i] < medRMS*0.65 {
			active[i] = true
			activeCount++
		}
	}

	// If nothing was detected or almost everything was active, return original
	if activeCount == 0 || activeCount > numFrames*9/10 {
		return samples, false
	}

	// Pad each active frame by +/- 1.5 seconds (3 frames) and bridge small gaps (<= 3s = 6 frames)
	padFrames := 3
	padded := make([]bool, numFrames)
	for i := 0; i < numFrames; i++ {
		if !active[i] {
			continue
		}
		lo := i - padFrames
		if lo < 0 {
			lo = 0
		}
		hi := i + padFrames
		if hi >= numFrames {
			hi = numFrames - 1
		}
		for k := lo; k <= hi; k++ {
			padded[k] = true
		}
	}

	// Collect contiguous [startFrame, endFrame] segments
	type seg struct{ start, end int }
	var segments []seg
	inSeg := false
	segStart := 0
	for i := 0; i < numFrames; i++ {
		if padded[i] && !inSeg {
			inSeg = true
			segStart = i
		} else if !padded[i] && inSeg {
			inSeg = false
			segments = append(segments, seg{segStart, i - 1})
		}
	}
	if inSeg {
		segments = append(segments, seg{segStart, numFrames - 1})
	}

	fadeSamples := sampleRate / 20 // 50 ms fade at segment boundaries
	var out []float64
	for _, s := range segments {
		loSample := s.start * winSize
		hiSample := (s.end + 1) * winSize
		if hiSample > len(samples) {
			hiSample = len(samples)
		}
		chunk := make([]float64, hiSample-loSample)
		copy(chunk, samples[loSample:hiSample])

		// Apply smooth fade-in / fade-out to avoid boundary clicks
		fLen := fadeSamples
		if fLen*2 > len(chunk) {
			fLen = len(chunk) / 2
		}
		for k := 0; k < fLen; k++ {
			g := float64(k) / float64(fLen)
			chunk[k] *= g
			chunk[len(chunk)-1-k] *= g
		}
		out = append(out, chunk...)
	}

	if len(out) == 0 {
		return samples, false
	}
	return out, true
}

// EnhanceVoiceAudio applies a speech-band bandpass filter (250Hz - 3400Hz) and
// dynamic windowed gain normalization to make weak/noisy FM voice transmissions clearer.
func EnhanceVoiceAudio(samples []float64, sampleRate int) []float64 {
	if len(samples) == 0 || sampleRate <= 0 {
		return samples
	}

	// 1. Speech bandpass filter (250 Hz HPF -> 3400 Hz LPF)
	hpf := newBiquadHighpass(sampleRate, 250.0, 0.7071)
	lpfCutoff := 3400.0
	if float64(sampleRate)*0.45 < lpfCutoff {
		lpfCutoff = float64(sampleRate) * 0.45
	}
	lpf := newBiquadLowpass(sampleRate, lpfCutoff, 0.7071)

	filtered := make([]float64, len(samples))
	for i, s := range samples {
		filtered[i] = lpf.process(hpf.process(s))
	}

	// 2. Dynamic windowed normalization (0.5s windows, smoothed gain transitions)
	winSize := sampleRate / 2
	if winSize < 64 {
		winSize = 64
	}
	numFrames := (len(filtered) + winSize - 1) / winSize
	targetRMS := 0.22
	maxGain := 8.0
	gains := make([]float64, numFrames)

	for i := 0; i < numFrames; i++ {
		start := i * winSize
		end := start + winSize
		if end > len(filtered) {
			end = len(filtered)
		}
		var sumSq, peak float64
		for j := start; j < end; j++ {
			v := filtered[j]
			sumSq += v * v
			av := math.Abs(v)
			if av > peak {
				peak = av
			}
		}
		rms := math.Sqrt(sumSq / float64(end-start))
		g := 1.0
		if rms > 0.005 {
			g = targetRMS / rms
			if g > maxGain {
				g = maxGain
			}
		} else {
			g = 1.0
		}
		if peak*g > 0.95 && peak > 0 {
			g = 0.95 / peak
		}
		gains[i] = g
	}

	// Smooth gains across adjacent frames and apply with soft clipping
	out := make([]float64, len(filtered))
	for i := 0; i < numFrames; i++ {
		gPrev := gains[i]
		if i > 0 {
			gPrev = 0.5 * (gains[i-1] + gains[i])
		}
		gNext := gains[i]
		if i+1 < numFrames {
			gNext = 0.5 * (gains[i] + gains[i+1])
		}
		start := i * winSize
		end := start + winSize
		if end > len(filtered) {
			end = len(filtered)
		}
		span := float64(end - start)
		for j := start; j < end; j++ {
			t := float64(j-start) / span
			g := gPrev*(1.0-t) + gNext*t
			v := filtered[j] * g
			if v > 0.98 {
				v = 0.98
			} else if v < -0.98 {
				v = -0.98
			}
			out[j] = v
		}
	}

	return out
}

// ProcessFMVoice extracts voice-active segments from FM demodulated audio and enhances speech clarity.
func ProcessFMVoice(samples []float64, sampleRate int) ([]float64, bool) {
	seg, extracted := ExtractVoiceSegments(samples, sampleRate)
	enhanced := EnhanceVoiceAudio(seg, sampleRate)
	return enhanced, extracted
}

// ProcessFMVoiceWAV reads a mono 16-bit WAV file, extracts and enhances FM voice segments,
// and writes the result to dstWavPath.
func ProcessFMVoiceWAV(srcWavPath, dstWavPath string) (bool, error) {
	samples, sampleRate, err := LoadWAV(srcWavPath)
	if err != nil {
		return false, err
	}
	enhanced, extracted := ProcessFMVoice(samples, sampleRate)
	int16Samples := ConvertFloatToINT16(enhanced)
	if err := SaveWAV(dstWavPath, sampleRate, int16Samples); err != nil {
		return extracted, err
	}
	return extracted, nil
}

func medianFloat64(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := make([]float64, len(vals))
	copy(cp, vals)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return 0.5 * (cp[mid-1] + cp[mid])
	}
	return cp[mid]
}
