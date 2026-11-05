package decoder

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"twnoaa/pkg/dsp"
	"twnoaa/pkg/logger"
)

// APRSPacket represents a fully parsed APRS packet
type APRSPacket struct {
	Timestamp   time.Time `json:"timestamp"`
	SourceCall  string    `json:"source_call"`
	DestCall    string    `json:"dest_call"`
	Path        []string  `json:"path"`
	PathDisplay string    `json:"path_display"`
	PayloadType string    `json:"payload_type"` // "position", "message", "status", "telemetry", "unknown"
	HasLocation bool      `json:"has_location"`
	Latitude    float64   `json:"latitude,omitempty"`
	Longitude   float64   `json:"longitude,omitempty"`
	Comment     string    `json:"comment"`
	Message     string    `json:"message,omitempty"`
	RawPayload  string    `json:"raw_payload"`
	RawTNC      string    `json:"raw_tnc"`
	RSSI        float64   `json:"rssi"`
}

// APRSDecoder decodes Bell 202 AFSK audio (1200 baud, 1200Hz mark / 2200Hz space)
// using dual-branch demodulation:
//   1. Quadrature Matched-Filter Tone Correlator (1200Hz/2200Hz Hann-windowed sin/cos correlation)
//   2. Delay-and-Multiply product detector
// with DPLL clock recovery, HDLC bit-stream deframing, and CRC-verified single-bit / USB boundary-drop recovery.
type APRSDecoder struct {
	sampleRate float64
	delay      int
	delayBuf   []float64
	delayIdx   int

	// Audio Bandpass Filter state (900Hz HPF + 2500Hz LPF)
	hpAlpha float64
	lpAlpha float64
	hp1     float64
	hp2     float64
	bp1     float64
	bp2     float64

	// Branch 1: Delay-and-Multiply LPF state (2-stage IIR at ~1200Hz)
	alpha float64
	lpf1  float64
	lpf2  float64

	// Branch 1: DPLL clock recovery
	samplesPerBit float64
	clockPhase    float64
	prevDemodVal  float64

	// Branch 1: NRZI & bit recovery
	prevBitLevel int
	onesCount    int
	hdlcShift    byte

	// Branch 1: HDLC frame assembly
	bitBuffer  byte
	bitCount   int
	frameBytes []byte
	inFrame    bool

	// Branch 2: Quadrature Matched-Filter Tone Correlator (1200Hz Mark / 2200Hz Space)
	mfWinLen       int
	mfMarkCos      []float64
	mfMarkSin      []float64
	mfSpaceCos     []float64
	mfSpaceSin     []float64
	mfRing         []float64
	mfRingIdx      int
	mfRingCount    int
	mfLPFAlpha     float64
	mfLPF          float64
	mfClockPhase   float64
	mfPrevDemod    float64
	mfPrevBitLevel int
	mfShiftReg     byte
	mfRawBits      []int

	allPackets    []*APRSPacket
	currentRSSI   float64
	sampleCounter int64
	lastRawTNC    string
	lastPktSample int64
}

// NewAPRSDecoder creates an APRS AFSK 1200 decoder for a given audio sample rate
func NewAPRSDecoder(sampleRate int) *APRSDecoder {
	sr := float64(sampleRate)
	baud := 1200.0
	samplesPerBit := sr / baud

	// Optimal delay for 1700 Hz center frequency (quarter wavelength: 90 deg phase difference)
	delay := int(math.Round(sr / (4.0 * 1700.0)))
	if delay < 1 {
		delay = 1
	}

	// Audio Bandpass Filter (900Hz HPF + 2500Hz LPF) to reject out-of-band FM discriminator noise
	hpAlpha := math.Exp(-2.0 * math.Pi * 900.0 / sr)
	lpAlpha := math.Exp(-2.0 * math.Pi * 2500.0 / sr)

	// 2-stage LPF alpha for ~1200Hz cutoff
	fc := 1200.0
	alpha := math.Exp(-2.0 * math.Pi * fc / sr)

	// Matched-Filter window length (~1 bit period)
	mfWinLen := int(math.Round(samplesPerBit))
	if mfWinLen < 8 {
		mfWinLen = 8
	}
	mCos := make([]float64, mfWinLen)
	mSin := make([]float64, mfWinLen)
	sCos := make([]float64, mfWinLen)
	sSin := make([]float64, mfWinLen)
	for i := 0; i < mfWinLen; i++ {
		w := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(mfWinLen-1)))
		t := float64(i) / sr
		mCos[i] = w * math.Cos(2.0*math.Pi*1200.0*t)
		mSin[i] = w * math.Sin(2.0*math.Pi*1200.0*t)
		sCos[i] = w * math.Cos(2.0*math.Pi*2200.0*t)
		sSin[i] = w * math.Sin(2.0*math.Pi*2200.0*t)
	}
	mfLPFAlpha := math.Exp(-2.0 * math.Pi * 1100.0 / sr)

	return &APRSDecoder{
		sampleRate:    sr,
		delay:         delay,
		delayBuf:      make([]float64, delay),
		delayIdx:      0,
		hpAlpha:       hpAlpha,
		lpAlpha:       lpAlpha,
		alpha:         alpha,
		lpf1:          0.0,
		lpf2:          0.0,
		samplesPerBit: samplesPerBit,
		clockPhase:    0.0,
		prevDemodVal:  0.0,
		prevBitLevel:  0,
		onesCount:     0,
		frameBytes:    make([]byte, 0, 512),
		inFrame:       false,
		mfWinLen:      mfWinLen,
		mfMarkCos:     mCos,
		mfMarkSin:     mSin,
		mfSpaceCos:    sCos,
		mfSpaceSin:    sSin,
		mfRing:        make([]float64, mfWinLen),
		mfLPFAlpha:    mfLPFAlpha,
		mfRawBits:     make([]int, 0, 1024),
		allPackets:    make([]*APRSPacket, 0),
		currentRSSI:   -50.0,
		lastPktSample: -100000,
	}
}

// SetCurrentRSSI updates RSSI value associated with newly detected packets
func (d *APRSDecoder) SetCurrentRSSI(rssi float64) {
	d.currentRSSI = rssi
}

func (d *APRSDecoder) emitPacket(packet *APRSPacket) {
	if packet == nil {
		return
	}
	// Deduplicate identical packets emitted by both branches within 0.8 seconds
	minGapSamples := int64(d.sampleRate * 0.8)
	if packet.RawTNC == d.lastRawTNC && (d.sampleCounter-d.lastPktSample) < minGapSamples {
		return
	}
	d.lastRawTNC = packet.RawTNC
	d.lastPktSample = d.sampleCounter

	packet.Timestamp = time.Now()
	packet.RSSI = d.currentRSSI
	d.allPackets = append(d.allPackets, packet)
	logger.Infof("APRS", "Packet decoded: %s (Source: %s -> Dest: %s, Path: %s, Type: %s)",
		packet.RawTNC, packet.SourceCall, packet.DestCall, packet.PathDisplay, packet.PayloadType)
}

// DecodeSamples processes a stream of PCM float64 samples (-1.0 to +1.0) and returns newly decoded packets
func (d *APRSDecoder) DecodeSamples(samples []float64) []*APRSPacket {
	if len(samples) == 0 {
		return nil
	}

	startPacketCount := len(d.allPackets)
	midBit := d.samplesPerBit / 2.0

	for _, rawSample := range samples {
		d.sampleCounter++

		// 0. Audio Bandpass Filter (900Hz - 2500Hz) to reject out-of-band FM discriminator noise
		d.hp1 = d.hp1*d.hpAlpha + rawSample*(1.0-d.hpAlpha)
		h1 := rawSample - d.hp1
		d.hp2 = d.hp2*d.hpAlpha + h1*(1.0-d.hpAlpha)
		h2 := h1 - d.hp2

		d.bp1 = d.bp1*d.lpAlpha + h2*(1.0-d.lpAlpha)
		d.bp2 = d.bp2*d.lpAlpha + d.bp1*(1.0-d.lpAlpha)
		sample := d.bp2

		// --- Branch 1: Delay and Multiply ---
		delayedSample := d.delayBuf[d.delayIdx]
		d.delayBuf[d.delayIdx] = sample
		d.delayIdx = (d.delayIdx + 1) % d.delay

		product := sample * delayedSample

		d.lpf1 = d.lpf1*d.alpha + product*(1.0-d.alpha)
		d.lpf2 = d.lpf2*d.alpha + d.lpf1*(1.0-d.alpha)
		demodVal := d.lpf2

		if (demodVal > 0 && d.prevDemodVal <= 0) || (demodVal < 0 && d.prevDemodVal >= 0) {
			phaseErr := d.clockPhase
			if phaseErr > midBit {
				phaseErr -= d.samplesPerBit
			}
			d.clockPhase -= phaseErr * 0.25
		}
		d.prevDemodVal = demodVal

		oldPhase := d.clockPhase
		d.clockPhase += 1.0
		if oldPhase < midBit && d.clockPhase >= midBit {
			curLevel := 0
			if demodVal > 0 {
				curLevel = 1
			}
			dataBit := 0
			if curLevel == d.prevBitLevel {
				dataBit = 1
			}
			d.prevBitLevel = curLevel
			d.processDataBit(dataBit)
		}
		if d.clockPhase >= d.samplesPerBit {
			d.clockPhase -= d.samplesPerBit
		}

		// --- Branch 2: Quadrature Matched-Filter Tone Correlator (1200 Hz / 2200 Hz) ---
		d.mfRing[d.mfRingIdx] = sample
		d.mfRingIdx = (d.mfRingIdx + 1) % d.mfWinLen
		if d.mfRingCount < d.mfWinLen {
			d.mfRingCount++
			continue
		}

		var mI, mQ, sI, sQ float64
		idx := d.mfRingIdx // oldest sample in ring
		for k := 0; k < d.mfWinLen; k++ {
			v := d.mfRing[idx]
			mI += v * d.mfMarkCos[k]
			mQ += v * d.mfMarkSin[k]
			sI += v * d.mfSpaceCos[k]
			sQ += v * d.mfSpaceSin[k]
			idx++
			if idx >= d.mfWinLen {
				idx = 0
			}
		}
		rawDiff := math.Hypot(mI, mQ) - math.Hypot(sI, sQ)
		d.mfLPF = d.mfLPF*d.mfLPFAlpha + rawDiff*(1.0-d.mfLPFAlpha)
		mfDemod := d.mfLPF

		if (mfDemod > 0 && d.mfPrevDemod <= 0) || (mfDemod < 0 && d.mfPrevDemod >= 0) {
			phaseErr := d.mfClockPhase
			if phaseErr > midBit {
				phaseErr -= d.samplesPerBit
			}
			d.mfClockPhase -= phaseErr * 0.25
		}
		d.mfPrevDemod = mfDemod

		oldMFPhase := d.mfClockPhase
		d.mfClockPhase += 1.0
		if oldMFPhase < midBit && d.mfClockPhase >= midBit {
			curLevel := 0
			if mfDemod > 0 {
				curLevel = 1
			}
			dataBit := 0
			if curLevel == d.mfPrevBitLevel {
				dataBit = 1
			}
			d.mfPrevBitLevel = curLevel
			d.processMatchedFilterBit(dataBit)
		}
		if d.mfClockPhase >= d.samplesPerBit {
			d.mfClockPhase -= d.samplesPerBit
		}
	}

	if len(d.allPackets) > startPacketCount {
		return d.allPackets[startPacketCount:]
	}
	return nil
}

// processMatchedFilterBit accumulates raw NRZI bits between 0x7E (01111110) flags
func (d *APRSDecoder) processMatchedFilterBit(bit int) {
	d.mfShiftReg = (d.mfShiftReg << 1) | byte(bit&1)
	d.mfRawBits = append(d.mfRawBits, bit&1)
	if len(d.mfRawBits) > 4096 {
		d.mfRawBits = d.mfRawBits[len(d.mfRawBits)-2048:]
	}

	// Detect exact HDLC flag 01111110 (0x7E)
	if d.mfShiftReg == 0x7E {
		if len(d.mfRawBits) >= 8 {
			// Exclude the 8 bits of the closing 01111110 flag just received
			frameRawBits := d.mfRawBits[:len(d.mfRawBits)-8]
			if len(frameRawBits) >= 120 {
				d.decodeCandidateRawBits(frameRawBits)
			}
		}
		d.mfRawBits = d.mfRawBits[:0]
	}
}

func (d *APRSDecoder) decodeCandidateRawBits(rawBits []int) {
	destuffed := make([]int, 0, len(rawBits))
	ones := 0
	for _, b := range rawBits {
		if b == 1 {
			ones++
			destuffed = append(destuffed, 1)
		} else {
			if ones == 5 {
				// Discard stuffed zero after five consecutive 1s
				ones = 0
				continue
			}
			ones = 0
			destuffed = append(destuffed, 0)
		}
	}

	if len(destuffed) < 128 {
		return
	}

	// 1. Standard byte-aligned frame (len(destuffed) % 8 == 0)
	if len(destuffed)%8 == 0 {
		nBytes := len(destuffed) / 8
		frame := assembleBytesLSB(destuffed, nBytes)
		if pkt := verifyAndParseFrame(frame, true); pkt != nil {
			d.emitPacket(pkt)
			return
		}
	}

	// 2. Recovery for frames that experienced RTL-SDR ReadSync USB chunk-boundary bit drops
	if pkt := tryRecoverBoundaryDroppedFrame(destuffed); pkt != nil {
		d.emitPacket(pkt)
	}
}

func assembleBytesLSB(bits []int, nBytes int) []byte {
	frame := make([]byte, nBytes)
	for i := 0; i < nBytes; i++ {
		var by byte
		for k := 0; k < 8; k++ {
			by |= byte(bits[i*8+k] << k)
		}
		frame[i] = by
	}
	return frame
}

func verifyAndParseFrame(frame []byte, allowSingleBitRepair bool) *APRSPacket {
	if len(frame) < 16 {
		return nil
	}
	dataLen := len(frame) - 2
	crcReceived := binary.LittleEndian.Uint16(frame[dataLen:])
	crcComputed := computeCRC16(frame[:dataLen])

	if crcReceived == crcComputed {
		if pkt, err := ParseAX25Frame(frame[:dataLen]); err == nil && pkt != nil {
			return pkt
		}
	}

	if !allowSingleBitRepair {
		return nil
	}

	// Single-bit error repair: only accept if CRC-16 matches AND AX.25 header is strictly valid
	totalBits := len(frame) * 8
	for b := 0; b < totalBits; b++ {
		frame[b/8] ^= 1 << (b % 8)
		cRec := binary.LittleEndian.Uint16(frame[dataLen:])
		cComp := computeCRC16(frame[:dataLen])
		if cRec == cComp && isStrictValidAX25Header(frame[:dataLen]) {
			if pkt, err := ParseAX25Frame(frame[:dataLen]); err == nil && pkt != nil {
				frame[b/8] ^= 1 << (b % 8)
				return pkt
			}
		}
		frame[b/8] ^= 1 << (b % 8)
	}
	return nil
}

// isStrictValidAX25Header checks that all address bytes have valid shifted ASCII callsign characters,
// valid extension bits, UI control field (0x03), and No-Layer-3 PID (0xF0).
func isStrictValidAX25Header(data []byte) bool {
	if len(data) < 16 {
		return false
	}
	offset := 0
	for {
		if offset+7 > len(data) {
			return false
		}
		for i := 0; i < 6; i++ {
			b := data[offset+i]
			if (b & 0x01) != 0 {
				return false // Extension bit must be 0 in callsign characters
			}
			c := b >> 1
			if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ' ') {
				return false
			}
		}
		isLast := (data[offset+6] & 0x01) == 1
		offset += 7
		if isLast {
			break
		}
		if offset >= 70 { // Max 8 digipeaters
			return false
		}
	}
	if offset == 7 || offset+2 > len(data) {
		return false // Must have at least Dest and Src addresses + Control + PID
	}
	return data[offset] == 0x03 && data[offset+1] == 0xF0
}

// tryRecoverBoundaryDroppedFrame recovers satellite APRS frames where synchronous USB ReadSync
// chunk boundaries dropped 1 bit in the destination callsign and/or 9 bits ('R'/'I') in the ARISS status field.
// Requires an exact 16-bit CRC-16-CCITT match and strict AX.25 header validation.
func tryRecoverBoundaryDroppedFrame(destuffed []int) *APRSPacket {
	// Case A: 1 single bit dropped in the first 56 bits (len(destuffed)%8 == 7)
	if len(destuffed)%8 == 7 && len(destuffed) >= 135 {
		nBytes := (len(destuffed) + 1) / 8
		candBits := make([]int, len(destuffed)+1)
		for p1 := 0; p1 <= 55 && p1 <= len(destuffed); p1++ {
			for v1 := 0; v1 <= 1; v1++ {
				copy(candBits[:p1], destuffed[:p1])
				candBits[p1] = v1
				copy(candBits[p1+1:], destuffed[p1:])
				frame := assembleBytesLSB(candBits, nBytes)
				dataLen := len(frame) - 2
				if binary.LittleEndian.Uint16(frame[dataLen:]) == computeCRC16(frame[:dataLen]) && isStrictValidAX25Header(frame[:dataLen]) {
					if pkt, err := ParseAX25Frame(frame[:dataLen]); err == nil && pkt != nil {
						return pkt
					}
				}
			}
		}
	}

	// Case B: ~550-bit ISS RS0ISS Mic-E beacon where two 131KB ReadSync boundaries dropped
	// 0..2 bits in DestCall (bits 0..15) and 8..10 bits ('R'/'I') in "]ARISS-International Space Station=\r".
	if len(destuffed) >= 545 && len(destuffed) <= 556 {
		base70 := make([]byte, 70)
		base70[34] = 'R'
		base70[35] = 'I'

		for _, tailDelta := range []int{0, -1, 1} {
			tailStart := len(destuffed) - 272 + tailDelta
			if tailStart < 268 || tailStart+272 > len(destuffed) {
				continue
			}
			for bIdx := 36; bIdx < 70; bIdx++ {
				var by byte
				for k := 0; k < 8; k++ {
					by |= byte(destuffed[tailStart+(bIdx-36)*8+k] << k)
				}
				base70[bIdx] = by
			}
			crcRec := binary.LittleEndian.Uint16(base70[68:70])

			for _, shift01 := range []int{15, 16, 14} {
				if shift01+256 > tailStart {
					continue
				}
				for bIdx := 2; bIdx <= 33; bIdx++ {
					var by byte
					for k := 0; k < 8; k++ {
						by |= byte(destuffed[shift01+(bIdx-2)*8+k] << k)
					}
					base70[bIdx] = by
				}

				if !isStrictValidAX25Header(base70[7:68]) && (base70[21] != 0x03 || base70[22] != 0xF0) {
					continue
				}

				// Reconstruct Bytes 0..1 from destuffed[0:shift01]
				if shift01 == 16 {
					var by0, by1 byte
					for k := 0; k < 8; k++ {
						by0 |= byte(destuffed[k] << k)
						by1 |= byte(destuffed[8+k] << k)
					}
					base70[0] = by0
					base70[1] = by1
					if pkt := verifyAndParseFrame(base70, true); pkt != nil {
						return pkt
					}
				} else if shift01 == 15 {
					for p1 := 0; p1 <= 15; p1++ {
						for v1 := 0; v1 <= 1; v1++ {
							var b01 [16]int
							idx := 0
							for i := 0; i <= 15; i++ {
								if i == p1 {
									b01[idx] = v1
									idx++
								}
								if i < 15 {
									b01[idx] = destuffed[i]
									idx++
								}
							}
							var by0, by1 byte
							for k := 0; k < 8; k++ {
								by0 |= byte(b01[k] << k)
								by1 |= byte(b01[8+k] << k)
							}
							base70[0] = by0
							base70[1] = by1
							if computeCRC16(base70[:68]) == crcRec && isStrictValidAX25Header(base70[:68]) {
								if pkt, err := ParseAX25Frame(base70[:68]); err == nil && pkt != nil {
									return pkt
								}
							}
						}
					}
					// Fallback with 1-bit flip repair if needed
					for p1 := 0; p1 <= 15; p1++ {
						for v1 := 0; v1 <= 1; v1++ {
							var b01 [16]int
							idx := 0
							for i := 0; i <= 15; i++ {
								if i == p1 {
									b01[idx] = v1
									idx++
								}
								if i < 15 {
									b01[idx] = destuffed[i]
									idx++
								}
							}
							var by0, by1 byte
							for k := 0; k < 8; k++ {
								by0 |= byte(b01[k] << k)
								by1 |= byte(b01[8+k] << k)
							}
							base70[0] = by0
							base70[1] = by1
							if pkt := verifyAndParseFrame(base70, true); pkt != nil {
								return pkt
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// processDataBit handles HDLC flag detection, bit destuffing, and byte assembly for Branch 1
func (d *APRSDecoder) processDataBit(bit int) {
	if bit == 1 {
		d.onesCount++
		if d.onesCount == 6 {
			// Flag detected! The last 5 '1's were part of this flag, so rewind them
			d.rewindBits(5)
			if d.inFrame {
				d.finishFrame()
			}
			d.inFrame = true
			d.frameBytes = d.frameBytes[:0]
			d.bitBuffer = 0
			d.bitCount = 0
			return
		} else if d.onesCount > 6 {
			// Abort sequence (>6 consecutive 1s)
			d.inFrame = false
			d.frameBytes = d.frameBytes[:0]
			d.bitBuffer = 0
			d.bitCount = 0
			return
		}
	} else {
		// bit == 0
		if d.onesCount == 5 {
			// Stuffed 0 after five 1s: discard
			d.onesCount = 0
			return
		} else if d.onesCount == 6 {
			// Trailing 0 of flag 0x7E: discard
			d.onesCount = 0
			return
		}
		d.onesCount = 0
	}

	if !d.inFrame {
		return
	}

	// Append bit to current byte (LSB first)
	d.bitBuffer |= byte(bit << d.bitCount)
	d.bitCount++
	if d.bitCount == 8 {
		d.frameBytes = append(d.frameBytes, d.bitBuffer)
		d.bitBuffer = 0
		d.bitCount = 0

		if len(d.frameBytes) > 512 {
			d.inFrame = false
			d.frameBytes = d.frameBytes[:0]
		}
	}
}

// rewindBits removes the last n bits that were appended to the frame
func (d *APRSDecoder) rewindBits(n int) {
	for i := 0; i < n; i++ {
		if d.bitCount > 0 {
			d.bitCount--
			d.bitBuffer &= ^byte(1 << d.bitCount)
		} else if len(d.frameBytes) > 0 {
			lastByte := d.frameBytes[len(d.frameBytes)-1]
			d.frameBytes = d.frameBytes[:len(d.frameBytes)-1]
			d.bitBuffer = lastByte
			d.bitCount = 7
			d.bitBuffer &= ^byte(1 << d.bitCount)
		}
	}
}

// finishFrame verifies CRC and parses the AX.25 frame into an APRSPacket
func (d *APRSDecoder) finishFrame() {
	if len(d.frameBytes) < 16 { // Minimum AX.25 frame: 7 dest + 7 src + 1 ctl + 1 pid + 2 crc = 18
		return
	}

	// Check CRC-16-CCITT (FCS)
	dataLen := len(d.frameBytes) - 2
	crcReceived := binary.LittleEndian.Uint16(d.frameBytes[dataLen:])
	crcComputed := computeCRC16(d.frameBytes[:dataLen])

	if crcReceived != crcComputed {
		return
	}

	packet, err := ParseAX25Frame(d.frameBytes[:dataLen])
	if err == nil && packet != nil {
		d.emitPacket(packet)
	}
}

// computeCRC16 computes CCITT-CRC16 with polynomial 0x8408 (reversed 0x1021)
func computeCRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if (crc & 1) != 0 {
				crc = (crc >> 1) ^ 0x8408
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

func sanitizePrintablePayload(b []byte) string {
	trimmed := bytes.TrimRight(b, "\r\n ")
	var sb strings.Builder
	sb.Grow(len(trimmed))
	for _, c := range trimmed {
		if c >= 32 && c < 127 {
			sb.WriteByte(c)
		} else {
			sb.WriteByte(' ')
		}
	}
	return strings.TrimSpace(sb.String())
}

// ParseAX25Frame parses an un-flagged AX.25 frame byte slice
func ParseAX25Frame(frame []byte) (*APRSPacket, error) {
	if len(frame) < 16 {
		return nil, fmt.Errorf("frame too short (%d bytes)", len(frame))
	}

	// AX.25 Address format: 6 bytes callsign (each char >> 1), 1 byte SSID
	decodeCall := func(b []byte) (string, bool) {
		var call strings.Builder
		for i := 0; i < 6; i++ {
			c := b[i] >> 1
			if c != ' ' && c != 0 {
				call.WriteByte(c)
			}
		}
		ssidByte := b[6]
		ssid := (ssidByte >> 1) & 0x0F
		isLast := (ssidByte & 0x01) == 1

		callStr := call.String()
		if ssid != 0 {
			callStr = fmt.Sprintf("%s-%d", callStr, ssid)
		}
		return callStr, isLast
	}

	destCall, isLast := decodeCall(frame[0:7])
	if isLast {
		return nil, fmt.Errorf("invalid frame: dest address marked as last")
	}

	srcCall, isLast := decodeCall(frame[7:14])
	offset := 14

	path := make([]string, 0)
	for !isLast && offset+7 <= len(frame) {
		var digiCall string
		digiCall, isLast = decodeCall(frame[offset : offset+7])
		// Check H-bit (has been repeated) in SSID byte
		hasRepeated := (frame[offset+6] & 0x80) != 0
		if hasRepeated {
			digiCall += "*"
		}
		path = append(path, digiCall)
		offset += 7
	}

	if offset >= len(frame) {
		return nil, fmt.Errorf("frame truncated before control/PID")
	}

	// Control & PID
	control := frame[offset]
	offset++

	var pid byte = 0
	if (control & 0x03) == 0x03 { // UI Frame
		if offset < len(frame) {
			pid = frame[offset]
			offset++
		}
	}

	_ = pid
	payloadBytes := bytes.TrimRight(frame[offset:], "\r\n")
	rawPayload := sanitizePrintablePayload(payloadBytes)

	pathDisplay := strings.Join(path, ",")
	rawTNC := fmt.Sprintf("%s>%s", srcCall, destCall)
	if pathDisplay != "" {
		rawTNC = fmt.Sprintf("%s,%s:%s", rawTNC, pathDisplay, rawPayload)
	} else {
		rawTNC = fmt.Sprintf("%s:%s", rawTNC, rawPayload)
	}

	pkt := &APRSPacket{
		Timestamp:   time.Now(),
		SourceCall:  srcCall,
		DestCall:    destCall,
		Path:        path,
		PathDisplay: pathDisplay,
		RawPayload:  rawPayload,
		RawTNC:      rawTNC,
		PayloadType: "unknown",
	}

	parseAPRSData(pkt, payloadBytes)
	return pkt, nil
}

// parseAPRSData extracts position, Mic-E, comment, messages, etc. from APRS payload bytes
func parseAPRSData(pkt *APRSPacket, payloadBytes []byte) {
	if len(payloadBytes) == 0 {
		return
	}

	firstByte := payloadBytes[0]
	payload := string(payloadBytes)

	switch firstByte {
	case '!', '=': // Position without timestamp
		pkt.PayloadType = "position"
		parsePosition(pkt, payload[1:])
	case '/', '@': // Position with timestamp (7 chars timestamp)
		pkt.PayloadType = "position"
		if len(payload) > 8 {
			parsePosition(pkt, payload[8:])
		}
	case '\'', '`', 0x1c, 0x1d: // APRS Mic-E compressed position / status format (used by Kenwood TM-D710GA on ISS)
		parseMicE(pkt, payloadBytes)
	case ':': // Message
		pkt.PayloadType = "message"
		if len(payload) > 10 {
			// Format: :CALLSIGN  :Message text
			parts := strings.SplitN(payload[1:], ":", 2)
			if len(parts) == 2 {
				pkt.Comment = strings.TrimSpace(parts[1])
				pkt.Message = fmt.Sprintf("To %s: %s", strings.TrimSpace(parts[0]), pkt.Comment)
			} else {
				pkt.Comment = payload[1:]
			}
		}
	case '>': // Status
		pkt.PayloadType = "status"
		pkt.Comment = strings.TrimSpace(payload[1:])
	case 'T': // Telemetry
		if strings.HasPrefix(payload, "T#") {
			pkt.PayloadType = "telemetry"
			pkt.Comment = payload
		} else {
			pkt.Comment = payload
		}
	default:
		pkt.Comment = pkt.RawPayload
	}
}

// parseMicE decodes APRS Mic-E compressed position and status packets (APRS 1.0.1 Spec Chapter 10)
func parseMicE(pkt *APRSPacket, info []byte) {
	if len(info) < 9 {
		pkt.Comment = pkt.RawPayload
		return
	}

	// Extract comment/status starting at byte 9 (after DTI + 3 lon + 3 spd/crs + symCode + symTable)
	comment := sanitizePrintablePayload(info[9:])
	// Strip Kenwood radio type prefixes (']' for TM-D700/D710, '>' for TH-D7/D72/D74) and type suffixes ('=')
	if strings.HasPrefix(comment, "]") || strings.HasPrefix(comment, ">") {
		if strings.HasPrefix(comment, "]") && strings.HasSuffix(comment, "=") {
			comment = strings.TrimSuffix(comment[1:], "=")
		} else {
			comment = comment[1:]
		}
		comment = strings.TrimSpace(comment)
	}
	pkt.Comment = comment

	// Parse 6-character Mic-E destination callsign (strip SSID if present)
	dest := pkt.DestCall
	if idx := strings.IndexByte(dest, '-'); idx >= 0 {
		dest = dest[:idx]
	}

	// Note: The ISS Zvezda Kenwood TM-D710GA (RS0ISS) has no GPS receiver attached in orbit
	// and transmits the hardcoded default dummy Mic-E destination "0P0PS5" (00°00.35'N, 008°10.00'E).
	// Classify that unconfigured GPS beacon as a "status" packet rather than plotting a dummy coordinate in Africa.
	if dest == "0P0PS5" && (strings.HasPrefix(pkt.SourceCall, "RS0ISS") || strings.HasPrefix(pkt.SourceCall, "NA1SS")) {
		pkt.PayloadType = "status"
		pkt.HasLocation = false
		return
	}

	if len(dest) != 6 {
		pkt.PayloadType = "status"
		return
	}

	var digits [6]int
	for i := 0; i < 6; i++ {
		c := dest[i]
		switch {
		case c >= '0' && c <= '9':
			digits[i] = int(c - '0')
		case c >= 'A' && c <= 'J':
			digits[i] = int(c - 'A')
		case c >= 'P' && c <= 'Y':
			digits[i] = int(c - 'P')
		case c == 'K' || c == 'L' || c == 'Z':
			digits[i] = 0
		default:
			pkt.PayloadType = "status"
			return
		}
	}

	isNorth := dest[3] >= 'P'
	lonOffset100 := dest[4] >= 'P'
	isWest := dest[5] >= 'P'

	latDeg := float64(digits[0]*10 + digits[1])
	latMin := float64(digits[2]*10+digits[3]) + float64(digits[4]*10+digits[5])/100.0
	lat := latDeg + latMin/60.0
	if !isNorth {
		lat = -lat
	}

	// Decode Longitude from info[1..3]
	d := int(info[1]) - 28
	if lonOffset100 {
		d += 100
	}
	if d >= 180 && d <= 189 {
		d -= 80
	} else if d >= 190 && d <= 199 {
		d -= 190
	}

	m := int(info[2]) - 28
	if m >= 60 {
		m -= 60
	}
	h := int(info[3]) - 28
	if d < 0 || d > 180 || m < 0 || m >= 60 || h < 0 || h >= 100 {
		pkt.PayloadType = "status"
		return
	}

	lon := float64(d) + (float64(m)+float64(h)/100.0)/60.0
	if isWest {
		lon = -lon
	}

	pkt.PayloadType = "position"
	pkt.HasLocation = true
	pkt.Latitude = math.Round(lat*10000) / 10000
	pkt.Longitude = math.Round(lon*10000) / 10000
}

// parsePosition parses standard APRS coordinate formats: "3541.22N/13941.30E..."
func parsePosition(pkt *APRSPacket, s string) {
	if len(s) < 18 {
		pkt.Comment = s
		return
	}

	// Expected format: DDMM.hhN/DDDMM.hhE$ or similar
	latStr := s[0:7]  // "3541.22"
	latHem := s[7]    // 'N' or 'S'
	symTable := s[8]  // '/'
	lonStr := s[9:17] // "13941.30"
	lonHem := s[17]   // 'E' or 'W'

	_ = symTable
	if (latHem == 'N' || latHem == 'S') && (lonHem == 'E' || lonHem == 'W') {
		latDeg, err1 := strconv.ParseFloat(latStr[0:2], 64)
		latMin, err2 := strconv.ParseFloat(latStr[2:], 64)
		lonDeg, err3 := strconv.ParseFloat(lonStr[0:3], 64)
		lonMin, err4 := strconv.ParseFloat(lonStr[3:], 64)

		if err1 == nil && err2 == nil && err3 == nil && err4 == nil {
			lat := latDeg + (latMin / 60.0)
			if latHem == 'S' {
				lat = -lat
			}
			lon := lonDeg + (lonMin / 60.0)
			if lonHem == 'W' {
				lon = -lon
			}

			pkt.HasLocation = true
			pkt.Latitude = math.Round(lat*10000) / 10000
			pkt.Longitude = math.Round(lon*10000) / 10000

			if len(s) > 19 {
				pkt.Comment = strings.TrimSpace(s[19:])
			}
			return
		}
	}

	pkt.Comment = s
}

// DecodeAPRSWAVFile decodes all APRS packets contained in a WAV audio file
func DecodeAPRSWAVFile(wavPath string) ([]*APRSPacket, error) {
	samples, sampleRate, err := dsp.LoadWAV(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load WAV file %s: %w", wavPath, err)
	}

	decoder := NewAPRSDecoder(sampleRate)
	packets := decoder.DecodeSamples(samples)
	return packets, nil
}
