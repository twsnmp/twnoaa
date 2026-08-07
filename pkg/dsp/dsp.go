package dsp

import (
	"encoding/binary"
	"math"
	"math/cmplx"
	"os"
)

type DSP struct {
	decimation int
	prevSample complex128
}

func NewDSP(sampleRate, targetAudioRate int) *DSP {
	dec := sampleRate / targetAudioRate
	if dec < 1 {
		dec = 1
	}
	return &DSP{
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
	audio := make([]float64, 0, len(iq)/d.decimation)
	for i := 0; i < len(iq); i++ {
		diff := iq[i] * cmplx.Conj(d.prevSample)
		phaseDiff := cmplx.Phase(diff) // -PI to +PI
		d.prevSample = iq[i]

		if i%d.decimation == 0 {
			audio = append(audio, phaseDiff/math.Pi)
		}
	}
	return audio
}

// CalculatePowerSpectrum computes a 128-point or 256-point FFT power spectrum for waterfall visualization
func CalculatePowerSpectrum(buf []byte, numBins int) []float64 {
	if len(buf) < numBins*2 {
		return make([]float64, numBins)
	}

	bins := make([]float64, numBins)
	step := (len(buf) / 2) / numBins

	for i := 0; i < numBins; i++ {
		idx := i * step
		r := (float64(buf[2*idx]) - 127.5) / 127.5
		im := (float64(buf[2*idx+1]) - 127.5) / 127.5
		power := 10 * math.Log10(r*r+im*im+1e-12)
		// Normalize to 0.0 - 1.0 range
		bins[i] = math.Max(0, math.Min(1.0, (power+40.0)/40.0))
	}

	return bins
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
