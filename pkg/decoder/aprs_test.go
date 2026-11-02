package decoder

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

// Helper: build AX.25 address field (7 bytes)
func encodeCall(call string, isLast bool) []byte {
	out := make([]byte, 7)
	parts := strings.Split(call, "-")
	base := strings.ToUpper(parts[0])
	ssid := 0
	if len(parts) > 1 {
		switch parts[1] {
		case "1":
			ssid = 1
		case "2":
			ssid = 2
		case "9":
			ssid = 9
		case "11":
			ssid = 11
		}
	}

	for i := 0; i < 6; i++ {
		if i < len(base) {
			out[i] = base[i] << 1
		} else {
			out[i] = ' ' << 1
		}
	}

	ssidByte := byte((ssid << 1) | 0x60) // reserved bits set
	if isLast {
		ssidByte |= 0x01
	}
	out[6] = ssidByte
	return out
}

// Helper: create raw AX.25 frame bytes with CRC
func buildTestAX25Frame(src, dest string, digis []string, payload string) []byte {
	var frame []byte
	frame = append(frame, encodeCall(dest, false)...)
	frame = append(frame, encodeCall(src, len(digis) == 0)...)
	for i, d := range digis {
		isLast := (i == len(digis)-1)
		frame = append(frame, encodeCall(d, isLast)...)
	}

	// Control: 0x03 (UI), PID: 0xF0 (No layer 3)
	frame = append(frame, 0x03, 0xF0)
	frame = append(frame, []byte(payload)...)

	// Append CRC-16
	crc := computeCRC16(frame)
	crcBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(crcBytes, crc)
	frame = append(frame, crcBytes...)

	return frame
}

// Helper: synthesize Bell 202 AFSK audio samples from AX.25 frame
func synthesizeAFSK(frame []byte, sampleRate int, preambleFlags, postambleFlags int) []float64 {
	var bitstream []int
	addBit := func(b int) {
		bitstream = append(bitstream, b)
	}

	// Preamble 0x7E flags (01111110)
	for f := 0; f < preambleFlags; f++ {
		for b := 0; b < 8; b++ {
			addBit((0x7E >> b) & 1)
		}
	}

	// Data with bitstuffing
	onesCount := 0
	for _, byteVal := range frame {
		for b := 0; b < 8; b++ {
			bit := int((byteVal >> b) & 1)
			addBit(bit)
			if bit == 1 {
				onesCount++
				if onesCount == 5 {
					addBit(0) // Stuff 0
					onesCount = 0
				}
			} else {
				onesCount = 0
			}
		}
	}

	// Postamble 0x7E flags
	for f := 0; f < postambleFlags; f++ {
		for b := 0; b < 8; b++ {
			addBit((0x7E >> b) & 1)
		}
	}

	sr := float64(sampleRate)
	samplesPerBit := sr / 1200.0
	toneLevel := 0 // 0 = 1200Hz, 1 = 2200Hz

	var samples []float64
	phase := 0.0
	currentSample := 0

	for bitIdx, bit := range bitstream {
		if bit == 0 {
			toneLevel = 1 - toneLevel // NRZI toggle on 0
		}
		freq := 1200.0
		if toneLevel == 1 {
			freq = 2200.0
		}

		endSample := int(math.Round(float64(bitIdx+1) * samplesPerBit))
		num := endSample - currentSample

		for s := 0; s < num; s++ {
			samples = append(samples, 0.8*math.Sin(phase))
			phase += 2.0 * math.Pi * freq / sr
			if phase > 2.0*math.Pi {
				phase -= 2.0 * math.Pi
			}
		}
		currentSample = endSample
	}

	return samples
}

func TestParseAX25FrameDirect(t *testing.T) {
	payload := "=3541.22N/13941.30E-Greeting from Tokyo 73 via ISS"
	frame := buildTestAX25Frame("JA1ABC-9", "APRS", []string{"ARISS"}, payload)

	// Strip CRC before passing to ParseAX25Frame
	data := frame[:len(frame)-2]
	pkt, err := ParseAX25Frame(data)
	if err != nil {
		t.Fatalf("ParseAX25Frame failed: %v", err)
	}

	if pkt.SourceCall != "JA1ABC-9" {
		t.Errorf("Expected SourceCall JA1ABC-9, got %s", pkt.SourceCall)
	}
	if pkt.DestCall != "APRS" {
		t.Errorf("Expected DestCall APRS, got %s", pkt.DestCall)
	}
	if len(pkt.Path) != 1 || !strings.HasPrefix(pkt.Path[0], "ARISS") {
		t.Errorf("Expected Path [ARISS], got %v", pkt.Path)
	}
	if pkt.PayloadType != "position" {
		t.Errorf("Expected PayloadType position, got %s", pkt.PayloadType)
	}
	if !pkt.HasLocation {
		t.Fatalf("Expected HasLocation true")
	}

	if pkt.Latitude < 35.68 || pkt.Latitude > 35.69 {
		t.Errorf("Expected Latitude around 35.687, got %f", pkt.Latitude)
	}
	if pkt.Longitude < 139.68 || pkt.Longitude > 139.69 {
		t.Errorf("Expected Longitude around 139.688, got %f", pkt.Longitude)
	}
	if !strings.Contains(pkt.Comment, "Greeting from Tokyo") {
		t.Errorf("Expected Comment to contain Greeting from Tokyo, got %s", pkt.Comment)
	}
}

func TestAPRSDecoderDemodulateAudio(t *testing.T) {
	sampleRate := 20800
	payload := "=3541.22N/13941.30E-Test APRS Signal"
	frame := buildTestAX25Frame("JA1ABC", "APRS", []string{"ARISS"}, payload)

	// Generate AFSK modulated audio with sufficient preambles for DPLL lock
	audio := synthesizeAFSK(frame, sampleRate, 32, 16)

	decoder := NewAPRSDecoder(sampleRate)
	packets := decoder.DecodeSamples(audio)

	if len(packets) == 0 {
		t.Fatalf("Failed to decode any APRS packets from synthesized audio")
	}

	pkt := packets[0]
	if pkt.SourceCall != "JA1ABC" {
		t.Errorf("Expected SourceCall JA1ABC, got %s", pkt.SourceCall)
	}
	if !pkt.HasLocation {
		t.Errorf("Expected HasLocation true, got false")
	}
	if !strings.Contains(pkt.Comment, "Test APRS Signal") {
		t.Errorf("Expected comment to contain Test APRS Signal, got %s", pkt.Comment)
	}
}

func TestAPRSMessageAndStatusPacket(t *testing.T) {
	sampleRate := 20800

	// 1. Message packet
	msgPayload := ":NA1SS    :Hello Space Station!"
	msgFrame := buildTestAX25Frame("JH1XYZ", "APRS", []string{"ARISS*"}, msgPayload)
	msgAudio := synthesizeAFSK(msgFrame, sampleRate, 24, 12)

	dec := NewAPRSDecoder(sampleRate)
	pkts := dec.DecodeSamples(msgAudio)
	if len(pkts) != 1 {
		t.Fatalf("Expected 1 message packet, got %d", len(pkts))
	}
	if pkts[0].PayloadType != "message" {
		t.Errorf("Expected payload_type message, got %s", pkts[0].PayloadType)
	}
	if !strings.Contains(pkts[0].Message, "Hello Space Station!") {
		t.Errorf("Expected message content, got %s", pkts[0].Message)
	}

	// 2. Status packet from ISS
	statusPayload := ">ARISS FM & APRS Digipeater active 73"
	statusFrame := buildTestAX25Frame("RS0ISS-11", "APRS", nil, statusPayload)
	statusAudio := synthesizeAFSK(statusFrame, sampleRate, 24, 12)

	dec2 := NewAPRSDecoder(sampleRate)
	pkts2 := dec2.DecodeSamples(statusAudio)
	if len(pkts2) != 1 {
		t.Fatalf("Expected 1 status packet, got %d", len(pkts2))
	}
	if pkts2[0].SourceCall != "RS0ISS-11" {
		t.Errorf("Expected source RS0ISS-11, got %s", pkts2[0].SourceCall)
	}
	if pkts2[0].PayloadType != "status" {
		t.Errorf("Expected payload_type status, got %s", pkts2[0].PayloadType)
	}
}
