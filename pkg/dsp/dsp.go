package dsp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/cmplx"
	"os"
	"sort"
)

type DSP struct {
	sampleRate int
	targetRate int
	decimation int
	prevSample complex128
	iqLPFState complex128 // IQ low-pass filter state
	iqDCI      float64    // Running DC estimate for I channel
	iqDCQ      float64    // Running DC estimate for Q channel
	dcState    float64    // Audio DC blocker state
}

func NewDSP(sampleRate, targetAudioRate int) *DSP {
	dec := sampleRate / targetAudioRate
	if dec < 1 {
		dec = 1
	}
	return &DSP{
		sampleRate: sampleRate,
		targetRate: targetAudioRate,
		decimation: dec,
	}
}

// ConvertIQToComplex converts 8-bit interleaved IQ bytes (0 to 255) to complex128 (-1.0 to +1.0)
func ConvertIQToComplex(buf []byte) []complex128 {
	samples := make([]complex128, len(buf)/2)
	for i := 0; i < len(samples); i++ {
		r := (float64(buf[2*i]) - 127.5) / 127.5
		im := (float64(buf[2*i+1]) - 127.5) / 127.5
		samples[i] = complex(r, im)
	}
	return samples
}

// ConvertFloatToINT16 converts float64 PCM samples (-1.0 to 1.0) to int16 PCM
func ConvertFloatToINT16(samples []float64) []int16 {
	out := make([]int16, len(samples))
	for i, s := range samples {
		if s > 1.0 {
			s = 1.0
		}
		if s < -1.0 {
			s = -1.0
		}
		out[i] = int16(s * 32767.0)
	}
	return out
}

// DemodulateFM demodulates FM IQ samples into clean audio PCM float64 (-1.0 to 1.0)
func (d *DSP) DemodulateFM(iq []complex128) []float64 {
	if len(iq) == 0 {
		return nil
	}

	// Target intermediate rate for FM demodulation (~104 kHz)
	intermediateRate := 104000
	iqDec := d.sampleRate / intermediateRate
	if iqDec < 1 {
		iqDec = 1
	}
	actualIQRate := d.sampleRate / iqDec
	audioDec := actualIQRate / d.targetRate
	if audioDec < 1 {
		audioDec = 1
	}

	// 1. Low-Pass Filter (~18 kHz cutoff for 36 kHz APT IF bandwidth) & Decimate raw 2.08 MHz IQ to 104 kHz rate
	alpha := math.Exp(-2.0 * math.Pi * 18000.0 / float64(d.sampleRate))
	oneMinusAlpha := 1.0 - alpha

	decimatedIQ := make([]complex128, 0, len(iq)/iqDec)
	for i := 0; i < len(iq); i++ {
		raw := iq[i]
		r := real(raw)
		im := imag(raw)

		// DC Offset removal
		d.iqDCI = d.iqDCI*0.999 + r*0.001
		d.iqDCQ = d.iqDCQ*0.999 + im*0.001
		cleanIQ := complex(r-d.iqDCI, im-d.iqDCQ)

		// Low-pass filter (~40 kHz cutoff)
		d.iqLPFState = complex(
			real(d.iqLPFState)*alpha+real(cleanIQ)*oneMinusAlpha,
			imag(d.iqLPFState)*alpha+imag(cleanIQ)*oneMinusAlpha,
		)

		if i%iqDec == 0 {
			decimatedIQ = append(decimatedIQ, d.iqLPFState)
		}
	}

	// 2. FM Quadrature Demodulation at 104 kHz rate
	scaleFactor := float64(actualIQRate) / (2.0 * math.Pi * 17000.0) // ±17 kHz max deviation to ±1.0 PCM
	audio104k := make([]float64, len(decimatedIQ))

	for i, sample := range decimatedIQ {
		if d.prevSample == 0 {
			d.prevSample = sample
			audio104k[i] = 0
			continue
		}

		i0 := real(d.prevSample)
		q0 := imag(d.prevSample)
		i1 := real(sample)
		q1 := imag(sample)

		// Quadrature atan2(I1*Q0 - Q1*I0, I1*I0 + Q1*Q0)
		num := i1*q0 - q1*i0
		den := i1*i0 + q1*q0
		d.prevSample = sample

		phaseDiff := math.Atan2(num, den) // [-pi, +pi]
		audio104k[i] = phaseDiff * scaleFactor
	}

	// 3. Audio Decimation & DC Blocker
	audioOut := make([]float64, 0, len(audio104k)/audioDec)
	for i := 0; i+audioDec <= len(audio104k); i += audioDec {
		var sum float64
		for k := 0; k < audioDec; k++ {
			sum += audio104k[i+k]
		}
		avg := sum / float64(audioDec)
		acAudio := avg - d.dcState
		d.dcState = d.dcState*0.995 + avg*0.005

		if acAudio > 1.0 {
			acAudio = 1.0
		}
		if acAudio < -1.0 {
			acAudio = -1.0
		}
		audioOut = append(audioOut, acAudio)
	}

	return audioOut
}

// fft implements a Radix-2 Cooley-Tukey Fast Fourier Transform
func fft(x []complex128) []complex128 {
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

	fftEven := fft(even)
	fftOdd := fft(odd)

	combined := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		angle := -2 * math.Pi * float64(k) / float64(n)
		twiddle := cmplx.Rect(1, angle) * fftOdd[k]
		combined[k] = fftEven[k] + twiddle
		combined[k+n/2] = fftEven[k] - twiddle
	}
	return combined
}

// CalculatePowerSpectrum computes an FFT-based power spectrum (with Hann windowing & fftshift) for waterfall visualization
func CalculatePowerSpectrum(buf []byte, numBins int) []float64 {
	if numBins < 16 {
		numBins = 128
	}

	numSamples := len(buf) / 2
	if numSamples < numBins {
		return make([]float64, numBins)
	}

	// Calculate DC offset
	var sumR, sumIm float64
	for i := 0; i < numSamples; i++ {
		sumR += (float64(buf[2*i]) - 127.5) / 127.5
		sumIm += (float64(buf[2*i+1]) - 127.5) / 127.5
	}
	meanR := sumR / float64(numSamples)
	meanIm := sumIm / float64(numSamples)

	// Average FFT over available blocks to smooth spectrum
	blocks := numSamples / numBins
	if blocks > 8 {
		blocks = 8
	}

	avgPower := make([]float64, numBins)

	for b := 0; b < blocks; b++ {
		fftInput := make([]complex128, numBins)
		for i := 0; i < numBins; i++ {
			idx := b*numBins + i
			r := (float64(buf[2*idx]) - 127.5) / 127.5 - meanR
			im := (float64(buf[2*idx+1]) - 127.5) / 127.5 - meanIm

			// Apply Hann Window
			w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(numBins-1)))
			fftInput[i] = complex(r*w, im*w)
		}

		fftOut := fft(fftInput)

		// Shift zero-frequency component to center of spectrum (fftshift)
		half := numBins / 2
		for i := 0; i < numBins; i++ {
			shiftedIdx := (i + half) % numBins
			p := cmplx.Abs(fftOut[shiftedIdx])
			pDb := 10 * math.Log10(p*p + 1e-12)
			avgPower[i] += pDb
		}
	}

	bins := make([]float64, numBins)
	for i := 0; i < numBins; i++ {
		pDb := avgPower[i] / float64(blocks)
		bins[i] = math.Max(0, math.Min(1.0, (pDb+40.0)/35.0))
	}

	centerBin := numBins / 2
	if centerBin > 0 && centerBin < numBins-1 {
		bins[centerBin] = (bins[centerBin-1] + bins[centerBin+1]) / 2.0
	}

	return bins
}

// CalculateRSSI computes the Root Mean Square (RMS) signal level in dBFS
func CalculateRSSI(buf []byte) float64 {
	if len(buf) < 2 {
		return -100.0
	}

	numSamples := len(buf) / 2
	var sumR, sumIm float64
	for i := 0; i < numSamples; i++ {
		sumR += (float64(buf[2*i]) - 127.5) / 127.5
		sumIm += (float64(buf[2*i+1]) - 127.5) / 127.5
	}
	meanR := sumR / float64(numSamples)
	meanIm := sumIm / float64(numSamples)

	var sumSq float64
	for i := 0; i < numSamples; i++ {
		r := (float64(buf[2*i]) - 127.5) / 127.5 - meanR
		im := (float64(buf[2*i+1]) - 127.5) / 127.5 - meanIm
		sumSq += r*r + im*im
	}

	meanSq := sumSq / float64(numSamples)
	if meanSq <= 0 {
		return -100.0
	}

	rms := math.Sqrt(meanSq / 2.0)
	dbfs := 20 * math.Log10(rms + 1e-6)
	if dbfs > 0 {
		dbfs = 0
	}
	if dbfs < -100 {
		dbfs = -100
	}
	return dbfs
}

// CalculateSignalAndNoise calculates peak signal power, average noise floor power, and SNR in dB
func CalculateSignalAndNoise(buf []byte) (signalDBFS, noiseDBFS, snrDB float64) {
	numBins := 128
	if len(buf)/2 < numBins {
		return -100, -100, 0
	}

	numSamples := len(buf) / 2
	blocks := numSamples / numBins
	if blocks > 8 {
		blocks = 8
	}

	avgPower := make([]float64, numBins)

	for b := 0; b < blocks; b++ {
		fftInput := make([]complex128, numBins)
		for i := 0; i < numBins; i++ {
			idx := b*numBins + i
			r := (float64(buf[2*idx]) - 127.5) / 127.5
			im := (float64(buf[2*idx+1]) - 127.5) / 127.5
			w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(numBins-1)))
			fftInput[i] = complex(r*w, im*w)
		}

		fftOut := fft(fftInput)

		for i := 0; i < numBins; i++ {
			p := cmplx.Abs(fftOut[i])
			pDb := 10 * math.Log10(p*p + 1e-12)
			avgPower[i] += pDb
		}
	}

	powers := make([]float64, numBins)
	for i := 0; i < numBins; i++ {
		powers[i] = avgPower[i] / float64(blocks)
	}

	centerBin := numBins / 2
	if centerBin > 0 && centerBin < numBins-1 {
		powers[centerBin] = (powers[centerBin-1] + powers[centerBin+1]) / 2.0
	}

	sort.Float64s(powers)

	noiseIdx := numBins / 5
	noiseDBFS = powers[noiseIdx]

	signalIdx := numBins * 95 / 100
	signalDBFS = powers[signalIdx]

	snrDB = signalDBFS - noiseDBFS
	if snrDB < 0 {
		snrDB = 0
	}

	return signalDBFS, noiseDBFS, snrDB
}

// WriteWAVHeader writes a standard 16-bit Mono PCM WAV file header
func WriteWAVHeader(w io.Writer, sampleRate, numSamples int) error {
	var header [44]byte

	copy(header[0:4], []byte("RIFF"))
	dataSize := uint32(numSamples * 2)
	fileSize := uint32(36 + dataSize)
	binary.LittleEndian.PutUint32(header[4:8], fileSize)
	copy(header[8:12], []byte("WAVE"))

	copy(header[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	byteRate := uint32(sampleRate * 2)
	binary.LittleEndian.PutUint32(header[28:32], byteRate)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)

	copy(header[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:44], dataSize)

	_, err := w.Write(header[:])
	return err
}

// SaveWAV saves int16 PCM samples to a 16-bit Mono WAV file
func SaveWAV(filePath string, sampleRate int, int16Samples []int16) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := WriteWAVHeader(f, sampleRate, len(int16Samples)); err != nil {
		return err
	}

	buf := make([]byte, len(int16Samples)*2)
	for i, s := range int16Samples {
		binary.LittleEndian.PutUint16(buf[i*2:(i+1)*2], uint16(s))
	}

	_, err = f.Write(buf)
	return err
}

// SaveAudioToWAV saves float64 PCM samples (-1.0 to 1.0) into a 16-bit Mono WAV file
func SaveAudioToWAV(filePath string, sampleRate int, pcmSamples []float64) error {
	int16Samples := ConvertFloatToINT16(pcmSamples)
	return SaveWAV(filePath, sampleRate, int16Samples)
}

// LoadWAV loads a 16-bit Mono WAV file into float64 PCM samples
func LoadWAV(filePath string) ([]float64, int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var header [44]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return nil, 0, fmt.Errorf("failed to read WAV header: %w", err)
	}

	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, 0, errors.New("invalid WAV header format")
	}

	sampleRate := int(binary.LittleEndian.Uint32(header[24:28]))
	dataLen := binary.LittleEndian.Uint32(header[40:44])

	rawBuf := make([]byte, dataLen)
	if _, err := io.ReadFull(f, rawBuf); err != nil && err != io.EOF {
		return nil, 0, fmt.Errorf("failed to read WAV audio payload: %w", err)
	}

	numSamples := len(rawBuf) / 2
	pcmSamples := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		val := int16(binary.LittleEndian.Uint16(rawBuf[i*2 : (i+1)*2]))
		pcmSamples[i] = float64(val) / 32767.0
	}

	return pcmSamples, sampleRate, nil
}
