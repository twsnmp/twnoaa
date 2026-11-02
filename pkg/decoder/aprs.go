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
// using robust Delay-and-Multiply quadrature product demodulation and DPLL clock recovery.
type APRSDecoder struct {
	sampleRate float64
	delay      int
	delayBuf   []float64
	delayIdx   int

	// LPF state (2-stage IIR at ~1200Hz)
	alpha float64
	lpf1  float64
	lpf2  float64

	// DPLL clock recovery
	samplesPerBit float64
	clockPhase    float64
	prevDemodVal  float64

	// NRZI & bit recovery
	prevBitLevel int
	onesCount    int
	hdlcShift    byte

	// HDLC frame assembly
	bitBuffer   byte
	bitCount    int
	frameBytes  []byte
	inFrame     bool
	allPackets  []*APRSPacket
	currentRSSI float64
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

	// 2-stage LPF alpha for ~1200Hz cutoff
	fc := 1200.0
	alpha := math.Exp(-2.0 * math.Pi * fc / sr)

	return &APRSDecoder{
		sampleRate:    sr,
		delay:         delay,
		delayBuf:      make([]float64, delay),
		delayIdx:      0,
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
		allPackets:    make([]*APRSPacket, 0),
		currentRSSI:   -50.0,
	}
}

// SetCurrentRSSI updates RSSI value associated with newly detected packets
func (d *APRSDecoder) SetCurrentRSSI(rssi float64) {
	d.currentRSSI = rssi
}

// DecodeSamples processes a stream of PCM float64 samples (-1.0 to +1.0) and returns newly decoded packets
func (d *APRSDecoder) DecodeSamples(samples []float64) []*APRSPacket {
	if len(samples) == 0 {
		return nil
	}

	startPacketCount := len(d.allPackets)

	for _, sample := range samples {
		// 1. Delay and Multiply
		delayedSample := d.delayBuf[d.delayIdx]
		d.delayBuf[d.delayIdx] = sample
		d.delayIdx = (d.delayIdx + 1) % d.delay

		product := sample * delayedSample

		// 2. 2-stage LPF: 1200Hz (Mark) produces positive output, 2200Hz (Space) produces negative output
		d.lpf1 = d.lpf1*d.alpha + product*(1.0-d.alpha)
		d.lpf2 = d.lpf2*d.alpha + d.lpf1*(1.0-d.alpha)
		demodVal := d.lpf2

		// 3. DPLL zero-crossing clock synchronization
		if (demodVal > 0 && d.prevDemodVal <= 0) || (demodVal < 0 && d.prevDemodVal >= 0) {
			// Zero-crossing represents bit transition boundary (ideal phase = 0)
			phaseErr := d.clockPhase
			if phaseErr > d.samplesPerBit/2.0 {
				phaseErr -= d.samplesPerBit
			}
			d.clockPhase -= phaseErr * 0.25 // DPLL loop gain
		}
		d.prevDemodVal = demodVal

		// Advance clock phase
		oldPhase := d.clockPhase
		d.clockPhase += 1.0

		// Sample at the middle of the bit (samplesPerBit / 2.0)
		midBit := d.samplesPerBit / 2.0
		if oldPhase < midBit && d.clockPhase >= midBit {
			// 4. Sample symbol at bit center
			curLevel := 0
			if demodVal > 0 {
				curLevel = 1
			}

			// NRZI decoding: transition => 0, no transition => 1
			dataBit := 0
			if curLevel == d.prevBitLevel {
				dataBit = 1
			} else {
				dataBit = 0
			}
			d.prevBitLevel = curLevel

			d.processDataBit(dataBit)
		}

		if d.clockPhase >= d.samplesPerBit {
			d.clockPhase -= d.samplesPerBit
		}
	}

	if len(d.allPackets) > startPacketCount {
		return d.allPackets[startPacketCount:]
	}
	return nil
}

// processDataBit handles HDLC flag detection, bit destuffing, and byte assembly
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
		packet.Timestamp = time.Now()
		packet.RSSI = d.currentRSSI
		d.allPackets = append(d.allPackets, packet)
		logger.Infof("APRS", "Packet decoded: %s (Source: %s -> Dest: %s, Path: %s, Type: %s)",
			packet.RawTNC, packet.SourceCall, packet.DestCall, packet.PathDisplay, packet.PayloadType)
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
	payloadBytes := frame[offset:]
	rawPayload := string(bytes.TrimSpace(payloadBytes))

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

	parseAPRSData(pkt, rawPayload)
	return pkt, nil
}

// parseAPRSData extracts position, comment, messages, etc. from APRS payload text
func parseAPRSData(pkt *APRSPacket, payload string) {
	if len(payload) == 0 {
		return
	}

	firstChar := payload[0]

	switch firstChar {
	case '!', '=': // Position without timestamp
		pkt.PayloadType = "position"
		parsePosition(pkt, payload[1:])
	case '/', '@': // Position with timestamp (7 chars timestamp)
		pkt.PayloadType = "position"
		if len(payload) > 8 {
			parsePosition(pkt, payload[8:])
		}
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
		pkt.Comment = payload[1:]
	case 'T': // Telemetry
		if strings.HasPrefix(payload, "T#") {
			pkt.PayloadType = "telemetry"
			pkt.Comment = payload
		} else {
			pkt.Comment = payload
		}
	default:
		pkt.Comment = payload
	}
}

// parsePosition parses standard APRS coordinate formats: "3541.22N/13941.30E..."
func parsePosition(pkt *APRSPacket, s string) {
	if len(s) < 18 {
		pkt.Comment = s
		return
	}

	// Expected format: DDMM.hhN/DDDMM.hhE$ or similar
	latStr := s[0:7]   // "3541.22"
	latHem := s[7]     // 'N' or 'S'
	symTable := s[8]   // '/'
	lonStr := s[9:17]  // "13941.30"
	lonHem := s[17]    // 'E' or 'W'

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
