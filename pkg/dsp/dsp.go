package dsp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/cmplx"
	"os"
)

// iqLPFAlpha: IQ low-pass filter coefficient.
// Cutoff ≈ (1-α)/(2π) × f_IQ ≈ 43 kHz, reducing noise bandwidth from 2.08 MHz to 43 kHz.
// This reduces FM discriminator phase-diff noise by factor (1-α) ≈ 17.7 dB,
// bringing CNR from ~6 dB to ~24 dB (above FM threshold of 10-12 dB).
const iqLPFAlpha = 0.87

type DSP struct {
	sampleRate int
	targetRate int
	decimation int
	prevSample complex128
	iqLPFState complex128 // IQ low-pass filter state (noise bandwidth reduction)
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

// ConvertIQToComplex converts 8-bit interleaved IQ bytes to complex128 (-1.0 to +1.0)
func ConvertIQToComplex(buf []byte) []complex128 {
	samples := make([]complex128, len(buf)/2)
	for i := 0; i < len(samples); i++ {
		r := (float64(buf[2*i]) - 127.5) / 127.5
		im := (float64(buf[2*i+1]) - 127.5) / 127.5
		samples[i] = complex(r, im)
	}
	return samples
}

// DemodulateFM demodulates FM IQ samples into audio PCM float64 (-1.0 to 1.0)
func (d *DSP) DemodulateFM(iq []complex128) []float64 {
	if len(iq) == 0 {
		return nil
	}

	if d.decimation <= 1 {
		audio := make([]float64, len(iq))
		for i := 0; i < len(iq); i++ {
			raw := iq[i]
			r := real(raw)
			im := imag(raw)
			d.iqDCI = d.iqDCI*0.999 + r*0.001
			d.iqDCQ = d.iqDCQ*0.999 + im*0.001
			cleanIQ := complex(r-d.iqDCI, im-d.iqDCQ)

			diff := cleanIQ * cmplx.Conj(d.prevSample)
			rawPhase := cmplx.Phase(diff) / math.Pi
			d.prevSample = cleanIQ

			acPhase := rawPhase - d.dcState
			d.dcState = d.dcState*0.995 + rawPhase*0.005

			if acPhase > 1.0 {
				acPhase = 1.0
			}
			if acPhase < -1.0 {
				acPhase = -1.0
			}
			audio[i] = acPhase
		}
		return audio
	}

	// Two-Stage Decimation Architecture:
	// Stage 1: Filter and decimate 2.08 MHz complex IQ down to intermediate ~104 kHz rate (iqDec = 20)
	// Stage 2: FM phase demodulation at 104 kHz rate (prevents phase-wrapping for ±17 kHz WFM deviation)
	// Stage 3: Decimate audio PCM from 104 kHz down to target 20.8 kHz rate (audioDec = 5)
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

	// IQ LPF alpha for ~40 kHz cutoff at raw sample rate
	alpha := math.Exp(-2.0 * math.Pi * 40000.0 / float64(d.sampleRate))
	oneMinusAlpha := 1.0 - alpha

	// Scale factor mapping ±17 kHz WFM peak deviation at actualIQRate to ±1.0 audio PCM full scale
	scaleFactor := float64(actualIQRate) / (2.0 * 17000.0)

	audio := make([]float64, 0, len(iq)/(iqDec*audioDec))

	i := 0
	for i+iqDec*audioDec <= len(iq) {
		var sumAudio float64
		for a := 0; a < audioDec; a++ {
			for j := 0; j < iqDec; j++ {
				raw := iq[i]
				i++

				// 1. IQ DC Offset Cancellation (High-pass filter at 0 Hz)
				r := real(raw)
				im := imag(raw)
				d.iqDCI = d.iqDCI*0.999 + r*0.001
				d.iqDCQ = d.iqDCQ*0.999 + im*0.001
				cleanIQ := complex(r-d.iqDCI, im-d.iqDCQ)

				// 2. IQ Low-Pass Filter (~40 kHz cutoff)
				d.iqLPFState = complex(
					real(d.iqLPFState)*alpha+real(cleanIQ)*oneMinusAlpha,
					imag(d.iqLPFState)*alpha+imag(cleanIQ)*oneMinusAlpha,
				)
			}

			// 3. FM Phase Demodulation at 104 kHz rate
			diff := d.iqLPFState * cmplx.Conj(d.prevSample)
			d.prevSample = d.iqLPFState
			rawPhase := cmplx.Phase(diff) / math.Pi

			sumAudio += rawPhase
		}

		// 4. Audio Decimation & DC Blocker
		avgAudio := sumAudio / float64(audioDec)
		acAudio := avgAudio - d.dcState
		d.dcState = d.dcState*0.995 + avgAudio*0.005

		val := acAudio * scaleFactor
		if val > 1.0 {
			val = 1.0
		}
		if val < -1.0 {
			val = -1.0
		}
		audio = append(audio, val)
	}

	return audio
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
		// Normalize FFT dBFS power to 0.0 - 1.0 range for waterfall color map
		bins[i] = math.Max(0, math.Min(1.0, (pDb+40.0)/35.0))
	}

	// Remove hardware DC spike artifact at center 0 Hz bin by interpolating adjacent bins
	centerBin := numBins / 2
	if centerBin > 0 && centerBin < numBins-1 {
		bins[centerBin] = (bins[centerBin-1] + bins[centerBin+1]) / 2.0
	}

	return bins
}

// CalculateRSSI computes the Root Mean Square (RMS) signal power in dBFS from interleaved IQ bytes
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

	var sumPower float64
	for i := 0; i < numSamples; i++ {
		r := (float64(buf[2*i]) - 127.5) / 127.5 - meanR
		im := (float64(buf[2*i+1]) - 127.5) / 127.5 - meanIm
		sumPower += r*r + im*im
	}
	avgPower := sumPower / float64(numSamples)
	if avgPower <= 1e-12 {
		return -100.0
	}
	return 10 * math.Log10(avgPower)
}

// CalculateSignalAndNoise computes Signal Level (dBFS), Noise Floor (dBFS), and SNR (dB) using FFT bins
func CalculateSignalAndNoise(buf []byte) (signal float64, noise float64, snr float64) {
	numBins := 128
	numSamples := len(buf) / 2
	if numSamples < numBins {
		return -100.0, -100.0, 0.0
	}

	var sumR, sumIm float64
	for i := 0; i < numSamples; i++ {
		sumR += (float64(buf[2*i]) - 127.5) / 127.5
		sumIm += (float64(buf[2*i+1]) - 127.5) / 127.5
	}
	meanR := sumR / float64(numSamples)
	meanIm := sumIm / float64(numSamples)

	fftInput := make([]complex128, numBins)
	for i := 0; i < numBins; i++ {
		r := (float64(buf[2*i]) - 127.5) / 127.5 - meanR
		im := (float64(buf[2*i+1]) - 127.5) / 127.5 - meanIm
		w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(numBins-1)))
		fftInput[i] = complex(r*w, im*w)
	}
	fftOut := fft(fftInput)

	bins := make([]float64, numBins)
	half := numBins / 2
	for i := 0; i < numBins; i++ {
		shiftedIdx := (i + half) % numBins
		p := cmplx.Abs(fftOut[shiftedIdx])
		bins[i] = 10 * math.Log10(p*p + 1e-12)
	}

	// Center signal band (around DC): bins 61..67 (center in fftshift)
	var signalSum float64
	sigCount := 0
	for i := 61; i <= 67; i++ {
		signalSum += math.Pow(10, bins[i]/10.0)
		sigCount++
	}
	sigPowerLin := signalSum / float64(sigCount)

	// Out-of-band Noise band: bins 10..40 and 88..118
	var noiseSum float64
	noiseCount := 0
	for i := 10; i <= 40; i++ {
		noiseSum += math.Pow(10, bins[i]/10.0)
		noiseCount++
	}
	for i := 88; i <= 118; i++ {
		noiseSum += math.Pow(10, bins[i]/10.0)
		noiseCount++
	}
	noisePowerLin := noiseSum / float64(noiseCount)

	signal = 10 * math.Log10(sigPowerLin + 1e-12)
	noise = 10 * math.Log10(noisePowerLin + 1e-12)
	snr = signal - noise
	if snr < 0 {
		snr = 0
	}
	return signal, noise, snr
}


// ConvertFloatToINT16 converts float64 PCM samples (-1.0 to 1.0) to int16 PCM samples (-32768 to 32767)
func ConvertFloatToINT16(samples []float64) []int16 {
	out := make([]int16, len(samples))
	for i, v := range samples {
		val := v * 32767.0
		if val > 32767.0 {
			val = 32767.0
		}
		if val < -32768.0 {
			val = -32768.0
		}
		out[i] = int16(val)
	}
	return out
}

// SaveWAV writes int16 audio samples to a WAV file with standard headers
func SaveWAV(filename string, sampleRate int, samples []int16) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	dataSize := uint32(len(samples) * 2)

	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], dataSize+36)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataSize)

	if _, err := file.Write(header); err != nil {
		return err
	}

	for _, s := range samples {
		if err := binary.Write(file, binary.LittleEndian, s); err != nil {
			return err
		}
	}

	return nil
}

// LoadWAV reads a WAV audio file and converts its PCM samples to float64 (-1.0 to 1.0) along with its sample rate
func LoadWAV(filename string) ([]float64, int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	header := make([]byte, 12)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, 0, fmt.Errorf("invalid WAV header: %w", err)
	}

	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("file is not a valid RIFF/WAVE file")
	}

	var numChannels uint16
	var sampleRate uint32
	var bitsPerSample uint16
	var audioFormat uint16
	var dataBytes []byte

	// Read chunks
	for {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(file, chunkHeader); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, 0, err
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		if chunkID == "fmt " {
			fmtBuf := make([]byte, chunkSize)
			if _, err := io.ReadFull(file, fmtBuf); err != nil {
				return nil, 0, fmt.Errorf("failed to read fmt chunk: %w", err)
			}
			if len(fmtBuf) < 16 {
				return nil, 0, fmt.Errorf("invalid fmt chunk size")
			}
			audioFormat = binary.LittleEndian.Uint16(fmtBuf[0:2])
			numChannels = binary.LittleEndian.Uint16(fmtBuf[2:4])
			sampleRate = binary.LittleEndian.Uint32(fmtBuf[4:8])
			bitsPerSample = binary.LittleEndian.Uint16(fmtBuf[14:16])
		} else if chunkID == "data" {
			dataBytes = make([]byte, chunkSize)
			if _, err := io.ReadFull(file, dataBytes); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, 0, fmt.Errorf("failed to read data chunk: %w", err)
			}
			break // Usually data is the main payload
		} else {
			// Skip unknown chunks
			if _, err := file.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return nil, 0, fmt.Errorf("failed to seek chunk %s: %w", chunkID, err)
			}
		}
	}

	if audioFormat != 1 && audioFormat != 3 { // 1 = PCM, 3 = IEEE Float
		return nil, 0, fmt.Errorf("unsupported WAV audio format %d (only uncompressed PCM/IEEE Float supported)", audioFormat)
	}

	if len(dataBytes) == 0 {
		return nil, 0, fmt.Errorf("no audio data found in WAV file")
	}

	if numChannels == 0 {
		numChannels = 1
	}

	bytesPerSample := int(bitsPerSample / 8)
	if bytesPerSample == 0 {
		bytesPerSample = 2 // default to 16-bit
	}

	totalSamples := len(dataBytes) / (bytesPerSample * int(numChannels))
	samples := make([]float64, totalSamples)

	for i := 0; i < totalSamples; i++ {
		var sum float64
		for ch := 0; ch < int(numChannels); ch++ {
			offset := (i*int(numChannels) + ch) * bytesPerSample
			if offset+bytesPerSample > len(dataBytes) {
				break
			}

			var val float64
			switch bitsPerSample {
			case 8:
				// Unsigned 8-bit PCM (0 to 255) -> -1.0 to 1.0
				u8 := dataBytes[offset]
				val = (float64(u8) - 128.0) / 128.0
			case 16:
				// Signed 16-bit PCM (-32768 to 32767)
				s16 := int16(binary.LittleEndian.Uint16(dataBytes[offset : offset+2]))
				val = float64(s16) / 32768.0
			case 32:
				if audioFormat == 3 {
					// 32-bit IEEE float
					bits := binary.LittleEndian.Uint32(dataBytes[offset : offset+4])
					val = float64(math.Float32frombits(bits))
				} else {
					// Signed 32-bit PCM
					s32 := int32(binary.LittleEndian.Uint32(dataBytes[offset : offset+4]))
					val = float64(s32) / 2147483648.0
				}
			default:
				// Fallback 16bit
				s16 := int16(binary.LittleEndian.Uint16(dataBytes[offset : offset+2]))
				val = float64(s16) / 32768.0
			}
			sum += val
		}
		// Average across channels to mono
		samples[i] = sum / float64(numChannels)
	}

	return samples, int(sampleRate), nil
}

