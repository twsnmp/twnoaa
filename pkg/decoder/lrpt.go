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
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"twnoaa/pkg/logger"
)

var stdLumQuant = [64]int{
	16, 11, 10, 16, 24, 40, 51, 61,
	12, 12, 14, 19, 26, 58, 60, 55,
	14, 13, 16, 24, 40, 57, 69, 56,
	14, 17, 22, 29, 51, 87, 80, 62,
	18, 22, 37, 56, 68, 109, 103, 77,
	24, 35, 55, 64, 81, 104, 113, 92,
	49, 64, 78, 87, 103, 121, 120, 101,
	72, 92, 95, 98, 112, 100, 103, 99,
}

var zigZagOrder = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

var dcLumBits = [16]int{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
var dcLumSyms = []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

var acLumBits = [16]int{0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 125}
var acLumSyms = []uint8{
	0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
	0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
	0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
	0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
	0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
	0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
	0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
	0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
	0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
	0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
	0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
	0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
	0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
	0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
	0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
	0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
	0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
	0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
	0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
	0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
	0xf9, 0xfa,
}

type lrptHuffTable struct {
	minCode [17]int
	maxCode [17]int
	valPtr  [17]int
	syms    []uint8
}

func buildLRPTHuffTable(bits [16]int, syms []uint8) *lrptHuffTable {
	ht := &lrptHuffTable{syms: syms}
	code := 0
	p := 0
	for l := 1; l <= 16; l++ {
		if bits[l-1] == 0 {
			ht.maxCode[l] = -1
		} else {
			ht.valPtr[l] = p
			ht.minCode[l] = code
			p += bits[l-1]
			code += bits[l-1]
			ht.maxCode[l] = code - 1
		}
		code <<= 1
	}
	return ht
}

type lrptBitReader struct {
	data []byte
	pos  int
}

func (br *lrptBitReader) readBit() (int, bool) {
	if br.pos >= len(br.data)*8 {
		return 0, false
	}
	b := (br.data[br.pos>>3] >> (7 - (br.pos & 7))) & 1
	br.pos++
	return int(b), true
}

func (br *lrptBitReader) readBits(n int) (int, bool) {
	val := 0
	for i := 0; i < n; i++ {
		b, ok := br.readBit()
		if !ok {
			return 0, false
		}
		val = (val << 1) | b
	}
	return val, true
}

func (br *lrptBitReader) decodeHuff(ht *lrptHuffTable) (uint8, bool) {
	code := 0
	for l := 1; l <= 16; l++ {
		b, ok := br.readBit()
		if !ok {
			return 0, false
		}
		code = (code << 1) | b
		if code <= ht.maxCode[l] {
			idx := ht.valPtr[l] + (code - ht.minCode[l])
			if idx >= 0 && idx < len(ht.syms) {
				return ht.syms[idx], true
			}
			return 0, false
		}
	}
	return 0, false
}

func extendJPEG(v, nBits int) int {
	if nBits == 0 {
		return 0
	}
	vt := 1 << (nBits - 1)
	if v < vt {
		return v + ((-1) << nBits) + 1
	}
	return v
}

var (
	lrptCosTable  [8][8]float64
	lrptAlphaNorm [8]float64
	rsExp         [510]uint8
	rsLog         [256]int
	rsToConv      [256]uint8
	rsToDual      [256]uint8
)

func init() {
	for u := 0; u < 8; u++ {
		if u == 0 {
			lrptAlphaNorm[u] = 1.0 / math.Sqrt2
		} else {
			lrptAlphaNorm[u] = 1.0
		}
		for x := 0; x < 8; x++ {
			lrptCosTable[x][u] = math.Cos((2.0*float64(x) + 1.0) * float64(u) * math.Pi / 16.0)
		}
	}

	poly := 0x187
	x := 1
	for i := 0; i < 255; i++ {
		rsExp[i] = uint8(x)
		rsExp[i+255] = uint8(x)
		rsLog[x] = i
		x <<= 1
		if (x & 0x100) != 0 {
			x ^= poly
		}
	}
	basis := [8]uint8{0x01, 0xC3, 0xA2, 0x51, 0xEB, 0xB6, 0x5B, 0xEE}
	for b := 0; b < 256; b++ {
		var c uint8
		for m := 0; m < 8; m++ {
			if ((b >> (7 - m)) & 1) != 0 {
				c ^= basis[m]
			}
		}
		rsToConv[b] = c
		rsToDual[c] = uint8(b)
	}
}

func gfMul(a, b uint8) uint8 {
	if a == 0 || b == 0 {
		return 0
	}
	return rsExp[rsLog[a]+rsLog[b]]
}

func gfInv(a uint8) uint8 {
	return rsExp[255-rsLog[a]]
}

func idct8x8(dct *[64]float64) [64]uint8 {
	var tmp [8][8]float64
	var out [64]uint8
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			var s float64
			for u := 0; u < 8; u++ {
				s += lrptAlphaNorm[u] * dct[y*8+u] * lrptCosTable[x][u]
			}
			tmp[y][x] = s
		}
	}
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			var s float64
			for v := 0; v < 8; v++ {
				s += lrptAlphaNorm[v] * tmp[v][x] * lrptCosTable[y][v]
			}
			val := math.Round(s/4.0 + 128.0)
			if val < 0 {
				val = 0
			} else if val > 255 {
				val = 255
			}
			out[y*8+x] = uint8(val)
		}
	}
	return out
}

func decode14MCUs(payload []byte, qFactor int, dcTab, acTab *lrptHuffTable) ([14][64]uint8, int) {
	var out [14][64]uint8
	if qFactor < 20 || qFactor > 100 {
		return out, 0
	}
	var fScale float64
	if qFactor > 20 && qFactor < 50 {
		fScale = 5000.0 / float64(qFactor)
	} else {
		fScale = 200.0 - 2.0*float64(qFactor)
	}
	var qTab [64]float64
	for k := 0; k < 64; k++ {
		v := math.Floor((fScale*float64(stdLumQuant[k]) + 50.0) / 100.0)
		if v < 1 {
			v = 1
		}
		qTab[k] = v
	}

	br := &lrptBitReader{data: payload}
	prevDC := 0
	for mcu := 0; mcu < 14; mcu++ {
		var dct [64]float64
		cat, ok := br.decodeHuff(dcTab)
		if !ok || cat > 11 {
			return out, mcu
		}
		bits, ok := br.readBits(int(cat))
		if !ok {
			return out, mcu
		}
		diff := extendJPEG(bits, int(cat))
		dc := prevDC + diff
		prevDC = dc
		dct[0] = float64(dc) * qTab[0]

		k := 1
		for k < 64 {
			rs, ok := br.decodeHuff(acTab)
			if !ok {
				return out, mcu
			}
			if rs == 0x00 {
				break
			}
			if rs == 0xF0 {
				k += 16
				continue
			}
			run := int(rs >> 4)
			size := int(rs & 0x0F)
			k += run
			if k >= 64 || size > 10 {
				return out, mcu
			}
			acBits, ok := br.readBits(size)
			if !ok {
				return out, mcu
			}
			acVal := extendJPEG(acBits, size)
			zz := zigZagOrder[k]
			dct[zz] = float64(acVal) * qTab[zz]
			k++
		}
		out[mcu] = idct8x8(&dct)
	}
	remBits := len(payload)*8 - br.pos
	if remBits < 0 || remBits > 24 {
		return out, 0
	}
	return out, 14
}

func parity8(x uint8) uint8 {
	x ^= x >> 4
	x ^= x >> 2
	x ^= x >> 1
	return x & 1
}

func generateCCSDSPRN(n int) []byte {
	prn := make([]byte, n)
	reg := uint8(0xFF)
	for i := 0; i < n; i++ {
		var b byte
		for bit := 0; bit < 8; bit++ {
			out := reg & 1
			b = (b << 1) | out
			fb := ((reg >> 0) ^ (reg >> 3) ^ (reg >> 5) ^ (reg >> 7)) & 1
			reg = (reg >> 1) | (fb << 7)
		}
		prn[i] = b
	}
	return prn
}

func viterbiDecodeNRZM(softBits []float64, invertG2 bool) []uint8 {
	numDataBits := len(softBits) / 2
	const numStates = 64
	var metric [numStates]float64
	var nextMetric [numStates]float64
	for s := 1; s < numStates; s++ {
		metric[s] = -1e9
	}
	paths := make([][numStates]uint8, numDataBits)

	var outG1, outG2 [numStates][2]float64
	for state := 0; state < numStates; state++ {
		for bit := 0; bit < 2; bit++ {
			reg := uint8((state << 1) | bit)
			g1 := parity8(reg & 0x4F)
			g2 := parity8(reg & 0x6D)
			if invertG2 {
				g2 ^= 1
			}
			outG1[state][bit] = 1.0 - 2.0*float64(g1)
			outG2[state][bit] = 1.0 - 2.0*float64(g2)
		}
	}

	for t := 0; t < numDataBits; t++ {
		s0 := softBits[2*t]
		s1 := softBits[2*t+1]
		for s := 0; s < numStates; s++ {
			nextMetric[s] = -1e12
		}
		for state := 0; state < numStates; state++ {
			m := metric[state]
			if m <= -1e8 {
				continue
			}
			for bit := 0; bit < 2; bit++ {
				nextState := ((state << 1) | bit) & 0x3F
				bm := s0*outG1[state][bit] + s1*outG2[state][bit]
				cand := m + bm
				if cand > nextMetric[nextState] {
					nextMetric[nextState] = cand
					paths[t][nextState] = uint8(state)
				}
			}
		}
		metric = nextMetric
	}

	bestState := 0
	bestM := metric[0]
	for s := 1; s < numStates; s++ {
		if metric[s] > bestM {
			bestM = metric[s]
			bestState = s
		}
	}

	bits := make([]uint8, numDataBits)
	st := uint8(bestState)
	for t := numDataBits - 1; t >= 0; t-- {
		bits[t] = st & 1
		st = paths[t][st]
	}

	nrzm := make([]uint8, numDataBits)
	var prev uint8
	for i, b := range bits {
		nrzm[i] = b ^ prev
		prev = b
	}
	return nrzm
}

func rsCorrectCADU(cadu []byte) (int, int, uint8) {
	totalFixed := 0
	validSubwords := 0
	var validMask uint8
	for inter := 0; inter < 4; inter++ {
		var c [255]uint8
		for k := 0; k < 255; k++ {
			c[k] = rsToConv[cadu[4+4*k+inter]]
		}
		var S [32]uint8
		allZero := true
		for j := 0; j < 32; j++ {
			rootPow := ((112 + j) * 11) % 255
			var s uint8
			for k := 0; k < 255; k++ {
				if s != 0 {
					s = rsExp[rsLog[s]+rootPow] ^ c[k]
				} else {
					s = c[k]
				}
			}
			S[j] = s
			if s != 0 {
				allZero = false
			}
		}
		if allZero {
			validSubwords++
			validMask |= 1 << uint(inter)
			continue
		}

		var C, B [33]uint8
		C[0] = 1
		B[0] = 1
		L := 0
		m := 1
		b := uint8(1)
		for n := 0; n < 32; n++ {
			d := S[n]
			for i := 1; i <= L; i++ {
				d ^= gfMul(C[i], S[n-i])
			}
			if d == 0 {
				m++
			} else if 2*L <= n {
				T := C
				scale := gfMul(d, gfInv(b))
				for i := 0; i+m < 33; i++ {
					C[i+m] ^= gfMul(scale, B[i])
				}
				L = n + 1 - L
				B = T
				b = d
				m = 1
			} else {
				scale := gfMul(d, gfInv(b))
				for i := 0; i+m < 33; i++ {
					C[i+m] ^= gfMul(scale, B[i])
				}
				m++
			}
		}
		if L == 0 || L > 16 {
			continue
		}

		var errIdx []int
		for k := 0; k < 255; k++ {
			p := 254 - k
			xkPow := (11 * p) % 255
			zPow := (255 - xkPow) % 255
			val := uint8(1)
			zCur := uint8(1)
			zStep := rsExp[zPow]
			for i := 1; i <= L; i++ {
				zCur = gfMul(zCur, zStep)
				val ^= gfMul(C[i], zCur)
			}
			if val == 0 {
				errIdx = append(errIdx, k)
			}
		}
		if len(errIdx) != L {
			continue
		}

		var mat [16][17]uint8
		for j := 0; j < L; j++ {
			rootPow := ((112 + j) * 11) % 255
			for i, k := range errIdx {
				pow := (rootPow * (254 - k)) % 255
				mat[j][i] = rsExp[pow]
			}
			mat[j][L] = S[j]
		}
		singular := false
		for col := 0; col < L; col++ {
			piv := col
			for piv < L && mat[piv][col] == 0 {
				piv++
			}
			if piv == L {
				singular = true
				break
			}
			mat[col], mat[piv] = mat[piv], mat[col]
			inv := gfInv(mat[col][col])
			for col2 := col; col2 <= L; col2++ {
				mat[col][col2] = gfMul(mat[col][col2], inv)
			}
			for r := 0; r < L; r++ {
				if r != col && mat[r][col] != 0 {
					f := mat[r][col]
					for col2 := col; col2 <= L; col2++ {
						mat[r][col2] ^= gfMul(f, mat[col][col2])
					}
				}
			}
		}
		if singular {
			continue
		}

		syndOK := true
		for j := L; j < 32; j++ {
			rootPow := ((112 + j) * 11) % 255
			var s uint8
			for i, k := range errIdx {
				pow := (rootPow * (254 - k)) % 255
				s ^= gfMul(rsExp[pow], mat[i][L])
			}
			if s != S[j] {
				syndOK = false
				break
			}
		}
		if !syndOK {
			continue
		}

		for i, k := range errIdx {
			c[k] ^= mat[i][L]
			cadu[4+4*k+inter] = rsToDual[c[k]]
		}
		totalFixed += L
		validSubwords++
		validMask |= 1 << uint(inter)
	}
	return totalFixed, validSubwords, validMask
}

type caduFrame struct {
	counter   uint32
	errCost   int
	margin    int
	validMask uint8
	data      []byte
}

// DecodeLRPT decodes a METEOR-M2 3/4 LRPT (72kbaud OQPSK) Raw IQ file using a 100% pure Go pipeline:
// - 4x time-domain FIR anti-aliasing decimation + impulse noise clipper
// - Frequency-domain Root-Raised-Cosine (alpha=0.6) matched filter + 4th-power carrier recovery
// - Soft-decision K=7 (0x4F, 0x6D) Viterbi + NRZ-M differential decoder
// - CCSDS CADU sync (0x1ACFFC1D), PRN descrambling, and interleaved RS(255,223) error correction
// - CCSDS Space Packet M_PDU reassembly and 14-MCU JPEG Huffman + IDCT image synthesis
func DecodeLRPT(iqPath string, sampleRate int, outDir, baseName string) (*DecodeResult, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

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

	totalSamples := int(info.Size() / 2)
	const N = 1 << 20 // ~0.504s window at 2.08 MSps
	if totalSamples >= N {
		res, ok, err := decodeLRPTPureGoImagery(f, totalSamples, sampleRate, outDir, baseName)
		if err == nil && ok {
			return res, nil
		}
	}

	return decodeLRPTSpectrogramFallback(f, int64(totalSamples), sampleRate, outDir, baseName)
}

func decodeLRPTPureGoImagery(f *os.File, totalSamples, sampleRate int, outDir, baseName string) (*DecodeResult, bool, error) {
	dcTab := buildLRPTHuffTable(dcLumBits, dcLumSyms)
	acTab := buildLRPTHuffTable(acLumBits, acLumSyms)
	prn := generateCCSDSPRN(1020)

	const N = 1 << 20
	stepSamples := N / 2
	nDec := N / 4
	fsDec := float64(sampleRate) / 4.0
	baud := 72000.20
	sps := fsDec / baud
	alpha := 0.6

	const firTaps = 16
	var fir [firTaps]float64
	fcNorm := 100000.0 / float64(sampleRate)
	var firSum float64
	for k := 0; k < firTaps; k++ {
		n := float64(k) - float64(firTaps-1)/2.0
		var sinc float64
		if math.Abs(n) < 1e-9 {
			sinc = 2.0 * fcNorm
		} else {
			sinc = math.Sin(2.0*math.Pi*fcNorm*n) / (math.Pi * n)
		}
		win := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(k)/float64(firTaps-1)))
		fir[k] = sinc * win
		firSum += fir[k]
	}
	for k := 0; k < firTaps; k++ {
		fir[k] /= firSum
	}

	// Precompute RRC filter weights
	maxBin := int(60000.0 * float64(nDec) / fsDec)
	f1 := baud * (1.0 - alpha) / 2.0
	f2 := baud * (1.0 + alpha) / 2.0
	rrcWeights := make([]float64, 2*maxBin+1)
	for k := -maxBin; k <= maxBin; k++ {
		freq := math.Abs(float64(k) * fsDec / float64(nDec))
		if freq <= f1 {
			rrcWeights[k+maxBin] = 1.0
		} else if freq <= f2 {
			rrcWeights[k+maxBin] = math.Sqrt(0.5 * (1.0 + math.Cos(math.Pi*(freq-f1)/(alpha*baud))))
		}
	}

	var starts []int
	for s := 0; s+N <= totalSamples; s += stepSamples {
		starts = append(starts, s)
	}

	var mu sync.Mutex
	caduMap := make(map[uint32]caduFrame)
	partialMap := make(map[uint32]caduFrame)

	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}
	jobs := make(chan int, len(starts))
	for _, s := range starts {
		jobs <- s
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, N*2)
			raw := make([]complex128, N)
			zDec := make([]complex128, nDec)
			specDec := make([]complex128, nDec)
			u4 := make([]complex128, nDec)
			specU4Filt := make([]complex128, nDec)
			zDerot := make([]complex128, nDec)
			quickIn := make([]complex128, 4096)

			for startSample := range jobs {
				if _, err := f.ReadAt(buf, int64(startSample*2)); err != nil {
					continue
				}

				// Fast 4096-point FFT pre-screen in the middle of the window to skip pure noise windows in <0.1ms
				midBase := (N / 2) * 2
				var qMeanR, qMeanI float64
				for i := 0; i < 4096; i++ {
					qMeanR += (float64(buf[midBase+2*i]) - 127.5) / 127.5
					qMeanI += (float64(buf[midBase+2*i+1]) - 127.5) / 127.5
				}
				qMeanR /= 4096.0
				qMeanI /= 4096.0
				for i := 0; i < 4096; i++ {
					r := (float64(buf[midBase+2*i])-127.5)/127.5 - qMeanR
					im := (float64(buf[midBase+2*i+1])-127.5)/127.5 - qMeanI
					quickIn[i] = complex(r, im)
				}
				qSpec := fftRadix2(quickIn)
				var inPow, outPow float64
				var inCnt, outCnt int
				for k := -512; k <= 512; k++ {
					if k >= -2 && k <= 2 {
						continue
					}
					idx := (k + 4096) % 4096
					p := real(qSpec[idx])*real(qSpec[idx]) + imag(qSpec[idx])*imag(qSpec[idx])
					freq := math.Abs(float64(k) * float64(sampleRate) / 4096.0)
					if freq <= 48000 {
						inPow += p
						inCnt++
					} else if freq >= 95000 && freq <= 240000 {
						outPow += p
						outCnt++
					}
				}
				if inCnt > 0 && outCnt > 0 {
					snrLinear := (inPow / float64(inCnt)) / (outPow/float64(outCnt) + 1e-12)
					if snrLinear < 1.15 { // < ~0.6 dB in-band excess
						continue
					}
				}

				var sumR, sumI float64
				for i := 0; i < N; i += 16 {
					sumR += (float64(buf[2*i]) - 127.5) / 127.5
					sumI += (float64(buf[2*i+1]) - 127.5) / 127.5
				}
				cnt := float64(N / 16)
				meanR := sumR / cnt
				meanI := sumI / cnt

				var rawSq float64
				for i := 0; i < N; i++ {
					r := (float64(buf[2*i])-127.5)/127.5 - meanR
					im := (float64(buf[2*i+1])-127.5)/127.5 - meanI
					raw[i] = complex(r, im)
					rawSq += r*r + im*im
				}
				rawRms := math.Sqrt(rawSq / float64(N))
				clipMag := 2.5 * rawRms
				for i, c := range raw {
					mag := cmplx.Abs(c)
					if mag > clipMag {
						raw[i] = c * complex(clipMag/mag, 0)
					}
				}

				for m := 0; m < nDec; m++ {
					base := m * 4
					if base+firTaps <= N {
						var accR, accI float64
						for k := 0; k < firTaps; k++ {
							c := raw[base+k]
							w := fir[k]
							accR += real(c) * w
							accI += imag(c) * w
						}
						zDec[m] = complex(accR, accI)
					} else {
						zDec[m] = 0
					}
				}

				specFullDec := fftRadix2(zDec)
				for i := range specDec {
					specDec[i] = 0
				}
				for k := -maxBin; k <= maxBin; k++ {
					idx := (k + nDec) % nDec
					specDec[idx] = cmplx.Conj(specFullDec[idx]) * complex(rrcWeights[k+maxBin], 0)
				}
				z := fftRadix2(specDec)
				for i := range z {
					c := cmplx.Conj(z[i]) / complex(float64(nDec), 0)
					z[i] = c
					mag := cmplx.Abs(c)
					if mag > 1e-9 {
						u := c / complex(mag, 0)
						u2 := u * u
						u4[i] = u2 * u2
					} else {
						u4[i] = 0
					}
				}

				specU4 := fftRadix2(u4)
				searchBins := int(32000.0 * float64(nDec) / fsDec)
				maxP4 := 0.0
				var sumP4 float64
				peakBin4 := 0
				for k := -searchBins; k <= searchBins; k++ {
					idx := (k + nDec) % nDec
					p := real(specU4[idx])*real(specU4[idx]) + imag(specU4[idx])*imag(specU4[idx])
					sumP4 += p
					if p > maxP4 {
						maxP4 = p
						peakBin4 = k
					}
				}
				avgP4 := sumP4 / float64(2*searchBins+1)
				if maxP4/(avgP4+1e-12) < 8.0 {
					continue
				}

				bwBins := int(math.Round(150.0 * float64(nDec) / fsDec))
				for i := range specU4Filt {
					specU4Filt[i] = 0
				}
				for dk := -bwBins; dk <= bwBins; dk++ {
					k := peakBin4 + dk
					idx := (k + nDec) % nDec
					specU4Filt[idx] = cmplx.Conj(specU4[idx])
				}
				w4 := fftRadix2(specU4Filt)
				var unwrapped4, prevPh4 float64
				for i := 0; i < nDec; i++ {
					c4 := cmplx.Conj(w4[i])
					ph4 := cmplx.Phase(c4)
					if i == 0 {
						unwrapped4 = ph4
					} else {
						d := ph4 - prevPh4
						for d > math.Pi {
							d -= 2 * math.Pi
						}
						for d < -math.Pi {
							d += 2 * math.Pi
						}
						unwrapped4 += d
					}
					prevPh4 = ph4
					phiC := (unwrapped4 - math.Pi) / 4.0
					zDerot[i] = z[i] * cmplx.Rect(1.0, -phiC)
				}

				for _, stateCfg := range []struct {
					qDir     float64
					invertG2 bool
					swapIQ   bool
				}{
					{qDir: -0.5, invertG2: false, swapIQ: true},
					{qDir: +0.5, invertG2: true, swapIQ: false},
				} {
					// Coherent 8-subsegment linear timing drift estimation (tau0 + spsActual)
					const nSegs = 8
					const segSyms = 4096
					var segPhasor [nSegs]complex128
					for seg := 0; seg < nSegs; seg++ {
						basePos := 1000.0 + float64(seg*segSyms)*sps
						for tStep := 0; tStep < 16; tStep++ {
							tauI := float64(tStep) / 16.0 * sps
							var sum4 float64
							for k := 0; k < segSyms; k += 4 {
								pos := basePos + float64(k)*sps + tauI
								if pos >= float64(nDec-1000) {
									break
								}
								vI := real(lrptInterp(zDerot, pos))
								vQ := imag(lrptInterp(zDerot, pos+stateCfg.qDir*sps))
								c := complex(vI, vQ)
								if m := cmplx.Abs(c); m > 1e-9 {
									c /= complex(m, 0)
								}
								c2 := c * c
								sum4 += -real(c2 * c2)
							}
							ang := 2.0 * math.Pi * float64(tStep) / 16.0
							segPhasor[seg] += complex(sum4*math.Cos(ang), sum4*math.Sin(ang))
						}
					}

					bestScore := -1.0
					bestDriftPerSeg := 0.0
					bestInitPhase := 0.0
					for d := -32; d <= 32; d++ {
						driftRadPerSeg := float64(d) * 0.25 * (2.0 * math.Pi / 16.0)
						var coherentSum complex128
						for seg := 0; seg < nSegs; seg++ {
							coherentSum += segPhasor[seg] * cmplx.Rect(1.0, -float64(seg)*driftRadPerSeg)
						}
						score := cmplx.Abs(coherentSum)
						if score > bestScore {
							bestScore = score
							bestDriftPerSeg = float64(d) * 0.25
							bestInitPhase = cmplx.Phase(coherentSum) * 16.0 / (2.0 * math.Pi)
							if bestInitPhase < 0 {
								bestInitPhase += 16.0
							}
						}
					}

					spsActual := sps + (bestDriftPerSeg/16.0)*sps/float64(segSyms)
					nBack := int(math.Floor((1000.0 + (bestInitPhase/16.0)*sps - 500.0) / spsActual))
					startPos := 1000.0 + (bestInitPhase/16.0)*sps - float64(nBack)*spsActual

					var syms []complex128
					var rmsSum float64
					for pos := startPos; pos < float64(nDec-500); pos += spsActual {
						vI := real(lrptInterp(zDerot, pos))
						vQ := imag(lrptInterp(zDerot, pos+stateCfg.qDir*spsActual))
						c := complex(vI, vQ)
						rmsSum += real(c)*real(c) + imag(c)*imag(c)
						syms = append(syms, c)
					}
					if len(syms) > 0 {
						rms := math.Sqrt(rmsSum / float64(len(syms)))
						if rms > 1e-9 {
							for i, c := range syms {
								c /= complex(rms, 0)
								if m := cmplx.Abs(c); m > 2.0 {
									c *= complex(2.0/m, 0)
								}
								syms[i] = c
							}
						}
					}

					soft := make([]float64, len(syms)*2)
					for i, s := range syms {
						if stateCfg.swapIQ {
							soft[2*i] = imag(s)
							soft[2*i+1] = real(s)
						} else {
							soft[2*i] = real(s)
							soft[2*i+1] = imag(s)
						}
					}
					bits := viterbiDecodeNRZM(soft, stateCfg.invertG2)

					var reg uint32
					for i := 0; i < len(bits); i++ {
						reg = (reg << 1) | uint32(bits[i])
						asmErr := hamming32(reg ^ 0x1ACFFC1D)
						if i >= 31 && asmErr <= 4 {
							frameStartBit := i - 31
							if frameStartBit+8192 <= len(bits) {
								cadu := make([]byte, 1024)
								for bIdx := 0; bIdx < 1024; bIdx++ {
									var b byte
									for bit := 0; bit < 8; bit++ {
										b = (b << 1) | bits[frameStartBit+bIdx*8+bit]
									}
									if bIdx >= 4 {
										b ^= prn[bIdx-4]
									}
									cadu[bIdx] = b
								}
								fixedBytes, validSubwords, validMask := rsCorrectCADU(cadu)
								vcid := uint8(cadu[5] & 0x3F)
								counter := (uint32(cadu[6]) << 16) | (uint32(cadu[7]) << 8) | uint32(cadu[8])
								if vcid == 5 && cadu[4] == 0x40 && validSubwords > 0 {
									margin := frameStartBit
									if rem := len(bits) - (frameStartBit + 8192); rem < margin {
										margin = rem
									}
									mu.Lock()
									if pPrev, pExists := partialMap[counter]; pExists && validMask != 0x0F {
										for inter := 0; inter < 4; inter++ {
											bitM := uint8(1 << uint(inter))
											if (pPrev.validMask&bitM) != 0 && (validMask&bitM) == 0 {
												for k := 0; k < 255; k++ {
													cadu[4+4*k+inter] = pPrev.data[4+4*k+inter]
												}
												validMask |= bitM
											}
										}
										validSubwords = hamming32(uint32(validMask))
									}
									partialMap[counter] = caduFrame{
										counter:   counter,
										validMask: validMask,
										data:      cadu,
									}
									// Admit to caduMap when all 4 subwords are RS-verified, or 3 subwords including counter/header (0x0D)
									if validMask == 0x0F || (validSubwords == 3 && (validMask&0x0D) == 0x0D) {
										errCost := (4-validSubwords)*200 + fixedBytes + asmErr*10
										prev, exists := caduMap[counter]
										if !exists || errCost < prev.errCost || (errCost == prev.errCost && margin > prev.margin) {
											caduMap[counter] = caduFrame{
												counter:   counter,
												errCost:   errCost,
												margin:    margin,
												validMask: validMask,
												data:      cadu,
											}
										}
									}
									mu.Unlock()
								}
								i += 8100
							}
						}
					}
				}
			}
		}()
	}
	wg.Wait()

	perfectCADUs := 0
	for _, cf := range caduMap {
		if cf.validMask == 0x0F {
			perfectCADUs++
		}
	}
	if perfectCADUs < 8 {
		return nil, false, nil
	}

	var counters []uint32
	for k := range caduMap {
		counters = append(counters, k)
	}
	sort.Slice(counters, func(i, j int) bool { return counters[i] < counters[j] })

	type mcuSegment struct {
		apid      uint16
		seqCnt    uint16
		ms        uint32
		mcuID     int
		validMCUs int
		blocks    [14][64]uint8
	}
	var segments []mcuSegment
	decodedByAPID := make(map[uint16]int)
	var partial []byte
	var prevCounter uint32
	var prevRSPerfect bool

	processPacket := func(pkt []byte, rsPerfect bool) {
		if len(pkt) < 21 {
			return
		}
		apid := ((uint16(pkt[0]) & 0x07) << 8) | uint16(pkt[1])
		if apid < 64 || apid > 69 {
			return
		}
		if pkt[17] != 0xFF || pkt[18] != 0xF0 {
			return
		}
		mcuID := int(pkt[14])
		if mcuID > 182 || mcuID%14 != 0 {
			return
		}
		qFactor := int(pkt[19])
		seqCnt := ((uint16(pkt[2]) & 0x3F) << 8) | uint16(pkt[3])
		msOfDay := (uint32(pkt[8]) << 24) | (uint32(pkt[9]) << 16) | (uint32(pkt[10]) << 8) | uint32(pkt[11])
		blocks, validMCUs := decode14MCUs(pkt[20:], qFactor, dcTab, acTab)
		if !rsPerfect {
			if validMCUs < 14 {
				return
			}
			for m := 0; m < validMCUs; m++ {
				var diffSum int
				blk := blocks[m]
				for y := 0; y < 7; y++ {
					for x := 0; x < 7; x++ {
						d1 := int(blk[y*8+x]) - int(blk[y*8+x+1])
						if d1 < 0 {
							d1 = -d1
						}
						d2 := int(blk[y*8+x]) - int(blk[(y+1)*8+x])
						if d2 < 0 {
							d2 = -d2
						}
						diffSum += d1 + d2
					}
				}
				if diffSum > 49*2*25 {
					validMCUs = m
					break
				}
				if m > 0 {
					var seamDiff int
					prevBlk := blocks[m-1]
					for y := 0; y < 8; y++ {
						d := int(prevBlk[y*8+7]) - int(blk[y*8+0])
						if d < 0 {
							d = -d
						}
						seamDiff += d
					}
					if seamDiff > 8*40 {
						validMCUs = m
						break
					}
				}
			}
		} else if validMCUs < 14 {
			validMCUs -= 2
		}
		if validMCUs > 0 {
			decodedByAPID[apid]++
			segments = append(segments, mcuSegment{
				apid:      apid,
				seqCnt:    seqCnt,
				ms:        msOfDay,
				mcuID:     mcuID,
				validMCUs: validMCUs,
				blocks:    blocks,
			})
		}
	}

	for _, c := range counters {
		cadu := caduMap[c].data
		rsPerfect := caduMap[c].errCost < 200
		fhp := ((int(cadu[12]) & 0x07) << 8) | int(cadu[13])
		if c == prevCounter+1 && len(partial) > 0 && fhp < 882 {
			partial = append(partial, cadu[14:14+fhp]...)
			if len(partial) >= 6 {
				pktLen := (int(partial[4]) << 8) | int(partial[5])
				if len(partial) >= 6+pktLen+1 {
					processPacket(partial[:6+pktLen+1], prevRSPerfect && rsPerfect)
				}
			}
		}
		partial = nil
		prevCounter = c
		prevRSPerfect = rsPerfect
		if fhp < 882 {
			pos := 14 + fhp
			for pos < 896 {
				if pos+6 > 896 {
					partial = append([]byte(nil), cadu[pos:896]...)
					break
				}
				pktLen := (int(cadu[pos+4]) << 8) | int(cadu[pos+5])
				totalPkt := 6 + pktLen + 1
				if pktLen <= 0 || pktLen > 2048 {
					break
				}
				if pos+totalPkt <= 896 {
					processPacket(cadu[pos:pos+totalPkt], rsPerfect)
					pos += totalPkt
				} else {
					partial = append([]byte(nil), cadu[pos:896]...)
					break
				}
			}
		}
	}

	if len(segments) < 14 {
		return nil, false, nil
	}

	// Clean outlier timestamps using local 5-packet median
	for i := range segments {
		var window []uint32
		for j := i - 2; j <= i+2; j++ {
			if j >= 0 && j < len(segments) {
				window = append(window, segments[j].ms)
			}
		}
		sort.Slice(window, func(a, b int) bool { return window[a] < window[b] })
		med := window[len(window)/2]
		if math.Abs(float64(int64(segments[i].ms)-int64(med))) > 2000 {
			segments[i].ms = med
		}
	}

	// Find robust minimum timestamp
	msList := make([]uint32, len(segments))
	for i, s := range segments {
		msList[i] = s.ms
	}
	sort.Slice(msList, func(i, j int) bool { return msList[i] < msList[j] })
	minMs := msList[len(msList)/50] // 2nd percentile to ignore any leading outlier

	type scanLine = lrptScanLine
	lineMap := make(map[int]*lrptScanLine)
	var rawKeys []int
	for _, s := range segments {
		key := int(math.Round(float64(int64(s.ms)-int64(minMs)) / 1230.85))
		if key < -10 || key > 1000 {
			continue
		}
		sl, ok := lineMap[key]
		if !ok {
			sl = &lrptScanLine{}
			lineMap[key] = sl
			rawKeys = append(rawKeys, key)
		}
		chIdx := -1
		switch s.apid {
		case 64:
			chIdx = 0
		case 65:
			chIdx = 1
		case 66, 67, 68, 69:
			chIdx = 2
		}
		if chIdx >= 0 {
			for m := 0; m < s.validMCUs; m++ {
				colMCU := s.mcuID + m
				if colMCU >= 0 && colMCU < 196 {
					sl.blocks[chIdx][colMCU] = s.blocks[m]
					sl.valid[chIdx][colMCU] = true
				}
			}
		}
	}
	sort.Ints(rawKeys)

	var lineKeys []int
	for _, k := range rawKeys {
		sl := lineMap[k]
		validCnt := 0
		for ch := 0; ch < 3; ch++ {
			for col := 0; col < 196; col++ {
				if sl.valid[ch][col] {
					validCnt++
				}
			}
		}
		if validCnt >= 84 {
			// If exactly 1 scanline was missed between two valid scanlines, insert it for smooth vertical interpolation
			if len(lineKeys) > 0 && k-lineKeys[len(lineKeys)-1] == 2 {
				midKey := k - 1
				if _, exists := lineMap[midKey]; !exists {
					lineMap[midKey] = &lrptScanLine{}
				}
				lineKeys = append(lineKeys, midKey)
			}
			lineKeys = append(lineKeys, k)
		}
	}

	numLines := len(lineKeys)
	if numLines < 3 {
		return nil, false, nil
	}

	fullImg, chanAImg, chanBImg, dayFraction := renderLRPTImages(lineMap, lineKeys, minMs)
	imgW := fullImg.Bounds().Dx()
	imgH := fullImg.Bounds().Dy()

	fullPath := filepath.Join(outDir, baseName+"_full.png")
	chanAPath := filepath.Join(outDir, baseName+"_chanA.png")
	chanBPath := filepath.Join(outDir, baseName+"_chanB.png")

	if err := writeRGBA(fullPath, fullImg); err != nil {
		return nil, false, err
	}
	_ = writeRGBA(chanAPath, chanAImg)
	_ = writeRGBA(chanBPath, chanBImg)

	logger.Infof("DECODER", "Pure Go LRPT decoded %d CADUs, packets=%v, day_fraction=%.2f -> %dx%d imagery: %s",
		len(caduMap), decodedByAPID, dayFraction, imgW, imgH, fullPath)

	return &DecodeResult{
		FullImagePath: fullPath,
		ChanAPath:     chanAPath,
		ChanBPath:     chanBPath,
		LineCount:     imgH,
		DecodeMode:    "lrpt",
		DayFraction:   dayFraction,
	}, true, nil
}

func hamming32(x uint32) int {
	c := 0
	for x != 0 {
		x &= x - 1
		c++
	}
	return c
}

func lrptInterp(x []complex128, pos float64) complex128 {
	idx := int(pos)
	frac := pos - float64(idx)
	if idx < 1 || idx+2 >= len(x) {
		if idx >= 0 && idx+1 < len(x) {
			return x[idx]*complex(1.0-frac, 0) + x[idx+1]*complex(frac, 0)
		}
		if idx >= 0 && idx < len(x) {
			return x[idx]
		}
		return 0
	}
	y0, y1, y2, y3 := x[idx-1], x[idx], x[idx+1], x[idx+2]
	c0 := y1
	c1 := complex(0.5, 0) * (y2 - y0)
	c2 := y0 - complex(2.5, 0)*y1 + complex(2.0, 0)*y2 - complex(0.5, 0)*y3
	c3 := complex(0.5, 0)*(y3-y0) + complex(1.5, 0)*(y1-y2)
	f := complex(frac, 0)
	return ((c3*f+c2)*f+c1)*f + c0
}

func fftRadix2(a []complex128) []complex128 {
	n := len(a)
	out := make([]complex128, n)
	copy(out, a)
	j := 0
	for i := 0; i < n; i++ {
		if i < j {
			out[i], out[j] = out[j], out[i]
		}
		m := n >> 1
		for m >= 1 && j >= m {
			j -= m
			m >>= 1
		}
		j += m
	}
	for s := 1; (1 << s) <= n; s++ {
		m := 1 << s
		halfM := m >> 1
		theta := -2.0 * math.Pi / float64(m)
		wM := cmplx.Rect(1.0, theta)
		for k := 0; k < n; k += m {
			w := complex(1.0, 0)
			for j := 0; j < halfM; j++ {
				t := w * out[k+j+halfM]
				u := out[k+j]
				out[k+j] = u + t
				out[k+j+halfM] = u - t
				w *= wM
			}
		}
	}
	return out
}

func decodeLRPTSpectrogramFallback(f *os.File, totalSamples int64, sampleRate int, outDir, baseName string) (*DecodeResult, error) {
	durationSec := float64(totalSamples) / float64(sampleRate)
	numLines := int(durationSec * 2.0)
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
			fftOut := fftRadix2(fftIn)
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

		centerBin := fftSize / 2
		zoomBins := 150
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
			if (x == 393 || x == 647) && y%4 < 2 {
				c = color.RGBA{R: 16, G: 185, B: 129, A: 255}
			}
			fullImg.SetRGBA(x, y, c)
			if x >= 65 && x < 65+909 {
				chanAImg.SetRGBA(x-65, y, c)
			}
		}

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

	logger.Infof("DECODER", "LRPT spectrogram fallback generated: %s", fullPath)
	return &DecodeResult{
		FullImagePath: fullPath,
		ChanAPath:     chanAPath,
		ChanBPath:     chanBPath,
		LineCount:     numLines,
		DecodeMode:    "lrpt_spectrogram",
		DayFraction:   0.0,
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

type lrptScanLine struct {
	blocks [3][196][64]uint8 // 0: APID 64 (Vis), 1: APID 65 (NIR), 2: APID 67/68 (IR)
	valid  [3][196]bool
}

func smoothstep(edge0, edge1, x float64) float64 {
	t := (x - edge0) / (edge1 - edge0)
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3.0 - 2.0*t)
}

func renderLRPTImages(lineMap map[int]*lrptScanLine, lineKeys []int, minMs uint32) (fullImg, chanAImg, chanBImg *image.RGBA, dayFraction float64) {
	numLines := len(lineKeys)
	if numLines == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1568, 8)), image.NewRGBA(image.Rect(0, 0, 1568, 8)), image.NewRGBA(image.Rect(0, 0, 1568, 8)), 0.0
	}

	// 1. Calculate un-stretched line mean for Vis channel (ch0) and smooth day weight w per line
	const visDarkLo = 18.0
	const visLitHi = 36.0

	lineVisMean := make([]float64, numLines)
	hasVis := make([]bool, numLines)
	var anyVis bool

	for rowIdx, k := range lineKeys {
		sl := lineMap[k]
		if sl == nil {
			continue
		}
		var sum float64
		var cnt int
		for col := 0; col < 196; col++ {
			if sl.valid[0][col] {
				for p := 0; p < 64; p++ {
					sum += float64(sl.blocks[0][col][p])
					cnt++
				}
			}
		}
		if cnt > 0 {
			lineVisMean[rowIdx] = sum / float64(cnt)
			hasVis[rowIdx] = true
			anyVis = true
		}
	}

	// Interpolate missing lineVisMean across missing lines
	if anyVis {
		for rowIdx := 0; rowIdx < numLines; rowIdx++ {
			if !hasVis[rowIdx] {
				upDist, downDist := 999999, 999999
				upVal, downVal := 0.0, 0.0
				for d := 1; rowIdx-d >= 0; d++ {
					if hasVis[rowIdx-d] {
						upDist = d
						upVal = lineVisMean[rowIdx-d]
						break
					}
				}
				for d := 1; rowIdx+d < numLines; d++ {
					if hasVis[rowIdx+d] {
						downDist = d
						downVal = lineVisMean[rowIdx+d]
						break
					}
				}
				if upDist < downDist {
					lineVisMean[rowIdx] = upVal
				} else if downDist < 999999 {
					lineVisMean[rowIdx] = downVal
				}
			}
		}
	}

	// Raw day-weight per line via smoothstep
	rawW := make([]float64, numLines)
	for rowIdx := 0; rowIdx < numLines; rowIdx++ {
		rawW[rowIdx] = smoothstep(visDarkLo, visLitHi, lineVisMean[rowIdx])
	}

	// Smooth day-weight w with moving average (radius ±16 lines) to eliminate banding along day/night terminator
	w := make([]float64, numLines)
	var sumW float64
	for rowIdx := 0; rowIdx < numLines; rowIdx++ {
		var s float64
		var c int
		for d := -16; d <= 16; d++ {
			idx := rowIdx + d
			if idx >= 0 && idx < numLines {
				s += rawW[idx]
				c++
			}
		}
		w[rowIdx] = s / float64(c)
		sumW += w[rowIdx]
	}
	dayFraction = sumW / float64(numLines)

	// 2. Compute 2nd and 98th percentile per channel
	// For ch0 (Vis) and ch1 (NIR), accumulate only from lit lines (w >= 0.5) to avoid night noise polluting daytime contrast.
	// For ch2 (IR), accumulate from all lines.
	var pLow, pHigh [3]float64
	for ch := 0; ch < 3; ch++ {
		var hist [256]int
		totalPix := 0
		for rowIdx, k := range lineKeys {
			if (ch == 0 || ch == 1) && w[rowIdx] < 0.5 {
				continue
			}
			sl := lineMap[k]
			if sl == nil {
				continue
			}
			for col := 0; col < 196; col++ {
				if sl.valid[ch][col] {
					blk := sl.blocks[ch][col]
					for p := 0; p < 64; p++ {
						hist[blk[p]]++
						totalPix++
					}
				}
			}
		}
		if totalPix == 0 {
			pLow[ch], pHigh[ch] = 0, 255
			continue
		}
		targetLow := totalPix * 2 / 100
		targetHigh := totalPix * 98 / 100
		acc := 0
		for v := 0; v < 256; v++ {
			acc += hist[v]
			if acc >= targetLow {
				pLow[ch] = float64(v)
				break
			}
		}
		acc = 0
		for v := 0; v < 256; v++ {
			acc += hist[v]
			if acc >= targetHigh {
				pHigh[ch] = float64(v)
				break
			}
		}
		if pHigh[ch]-pLow[ch] < 20 {
			pLow[ch] = 0
			pHigh[ch] = 255
		}
	}

	stretch := func(v uint8, ch int) float64 {
		norm := (float64(v) - pLow[ch]) / (pHigh[ch] - pLow[ch])
		if norm < 0 {
			return 0
		}
		if norm > 1 {
			return 1
		}
		return norm
	}

	imgW := 1568
	imgH := numLines * 8
	fullImg = image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	chanAImg = image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	chanBImg = image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	// 3. Resolve missing MCU blocks using smooth boundary-edge vertical interpolation
	// + co-registered cross-channel detail transfer (applied when daytime detail is present)
	resolved := make([][3][196][64]uint8, numLines)
	for rowIdx, key := range lineKeys {
		sl := lineMap[key]
		if sl == nil {
			continue
		}
		for ch := 0; ch < 3; ch++ {
			for colMCU := 0; colMCU < 196; colMCU++ {
				if sl.valid[ch][colMCU] {
					resolved[rowIdx][ch][colMCU] = sl.blocks[ch][colMCU]
					continue
				}
				upIdx, downIdx := -1, -1
				for d := 1; d <= 5; d++ {
					if upIdx < 0 && rowIdx-d >= 0 && key-lineKeys[rowIdx-d] <= 5 && lineMap[lineKeys[rowIdx-d]].valid[ch][colMCU] {
						upIdx = rowIdx - d
					}
					if downIdx < 0 && rowIdx+d < numLines && lineKeys[rowIdx+d]-key <= 5 && lineMap[lineKeys[rowIdx+d]].valid[ch][colMCU] {
						downIdx = rowIdx + d
					}
				}
				var vInterp [64]float64
				hasVInterp := false
				if upIdx >= 0 && downIdx >= 0 {
					bUp := lineMap[lineKeys[upIdx]].blocks[ch][colMCU]
					bDown := lineMap[lineKeys[downIdx]].blocks[ch][colMCU]
					totalSteps := float64((downIdx-upIdx-1)*8 + 9)
					for by := 0; by < 8; by++ {
						alpha := float64((rowIdx-upIdx-1)*8+by+1) / totalSteps
						for bx := 0; bx < 8; bx++ {
							vInterp[by*8+bx] = (1.0-alpha)*float64(bUp[7*8+bx]) + alpha*float64(bDown[0*8+bx])
						}
					}
					hasVInterp = true
				} else if upIdx >= 0 {
					bUp := lineMap[lineKeys[upIdx]].blocks[ch][colMCU]
					for by := 0; by < 8; by++ {
						for bx := 0; bx < 8; bx++ {
							vInterp[by*8+bx] = float64(bUp[7*8+bx])
						}
					}
					hasVInterp = true
				} else if downIdx >= 0 {
					bDown := lineMap[lineKeys[downIdx]].blocks[ch][colMCU]
					for by := 0; by < 8; by++ {
						for bx := 0; bx < 8; bx++ {
							vInterp[by*8+bx] = float64(bDown[0*8+bx])
						}
					}
					hasVInterp = true
				}

				// Cross-channel optical texture transfer (only when line is lit, w >= 0.5)
				otherOpt := 1 - ch
				if (ch == 0 || ch == 1) && w[rowIdx] >= 0.5 && sl.valid[otherOpt][colMCU] {
					bOther := sl.blocks[otherOpt][colMCU]
					var meanOther, meanSelf float64
					for p := 0; p < 64; p++ {
						meanOther += float64(bOther[p])
						meanSelf += vInterp[p]
					}
					meanOther /= 64.0
					meanSelf /= 64.0
					scaleRatio := (pHigh[ch] - pLow[ch]) / (pHigh[otherOpt] - pLow[otherOpt] + 1e-6)
					if !hasVInterp {
						meanSelf = pLow[ch] + (meanOther-pLow[otherOpt])*scaleRatio
					}
					for p := 0; p < 64; p++ {
						val := meanSelf + (float64(bOther[p])-meanOther)*scaleRatio
						if val < 0 {
							val = 0
						} else if val > 255 {
							val = 255
						}
						resolved[rowIdx][ch][colMCU][p] = uint8(math.Round(val))
					}
				} else if hasVInterp {
					for p := 0; p < 64; p++ {
						resolved[rowIdx][ch][colMCU][p] = uint8(math.Max(0, math.Min(255, math.Round(vInterp[p]))))
					}
				} else if sl.valid[0][colMCU] {
					resolved[rowIdx][ch][colMCU] = sl.blocks[0][colMCU]
				} else if sl.valid[1][colMCU] {
					resolved[rowIdx][ch][colMCU] = sl.blocks[1][colMCU]
				} else if sl.valid[2][colMCU] {
					bIR := sl.blocks[2][colMCU]
					for p := 0; p < 64; p++ {
						invIR := 1.0 - stretch(bIR[p], 2)
						val := pLow[ch] + invIR*(pHigh[ch]-pLow[ch])
						resolved[rowIdx][ch][colMCU][p] = uint8(math.Max(0, math.Min(255, math.Round(val))))
					}
				}
			}
		}
	}

	// 4. Compose images and apply ascending rotation
	ascendingPass := minMs >= 7200000 && minMs <= 36000000

	for rowIdx := 0; rowIdx < numLines; rowIdx++ {
		weight := w[rowIdx]
		for colMCU := 0; colMCU < 196; colMCU++ {
			bVis := resolved[rowIdx][0][colMCU]
			bNIR := resolved[rowIdx][1][colMCU]
			bIR := resolved[rowIdx][2][colMCU]
			for by := 0; by < 8; by++ {
				y := rowIdx*8 + by
				for bx := 0; bx < 8; bx++ {
					x := colMCU*8 + bx
					dstX, dstY := x, y
					if ascendingPass {
						dstX = imgW - 1 - x
						dstY = imgH - 1 - y
					}
					p := by*8 + bx
					vis := stretch(bVis[p], 0)
					nir := stretch(bNIR[p], 1)
					ir := stretch(bIR[p], 2)

					// Chan A (Visible)
					vA := uint8(math.Round(vis * 255.0))
					// Chan B (Thermal IR: inverted so cold cloud tops appear white)
					vB := uint8(math.Round((1.0 - ir) * 255.0))
					chanAImg.SetRGBA(dstX, dstY, color.RGBA{R: vA, G: vA, B: vA, A: 255})
					chanBImg.SetRGBA(dstX, dstY, color.RGBA{R: vB, G: vB, B: vB, A: 255})

					// Daytime false-color RGB (MSU-MR 1-2-1 + ocean/cloud enhancement)
					rDay := 0.80*nir + 0.20*vis
					gDay := 0.60*nir + 0.40*vis
					bDay := vis
					if nir < 0.45 && vis < 0.45 {
						waterFactor := (0.45 - nir) / 0.45
						rDay *= 1.0 - 0.35*waterFactor
						gDay = gDay*(1.0-0.15*waterFactor) + 0.06*waterFactor
						bDay = bDay + 0.22*waterFactor
					}

					// Nighttime thermal IR (inverted grayscale: cold clouds = white, warm sea = dark)
					nightVal := 1.0 - ir

					// Smooth blend between daytime false-color and nighttime thermal IR
					rF := weight*rDay + (1.0-weight)*nightVal
					gF := weight*gDay + (1.0-weight)*nightVal
					bF := weight*bDay + (1.0-weight)*nightVal

					r8 := uint8(math.Max(0, math.Min(255, math.Round(rF*255.0))))
					g8 := uint8(math.Max(0, math.Min(255, math.Round(gF*255.0))))
					b8 := uint8(math.Max(0, math.Min(255, math.Round(bF*255.0))))
					fullImg.SetRGBA(dstX, dstY, color.RGBA{R: r8, G: g8, B: b8, A: 255})
				}
			}
		}
	}

	return fullImg, chanAImg, chanBImg, dayFraction
}
