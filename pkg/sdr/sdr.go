package sdr

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"twnoaa/pkg/logger"

	rtl "github.com/jpoirier/gortlsdr"
)

type DeviceInfo struct {
	Count     int    `json:"count"`
	Name      string `json:"name"`
	IsPresent bool   `json:"is_present"`
}

type SDR struct {
	mu              sync.Mutex
	device          *rtl.Context
	centerFreq      int
	sampleRate      int
	gain            int
	autoGain        bool
	biasTee         bool
	isStreaming     bool
	simulationMode  bool
	stopChan        chan struct{}
	dataSubscribers []chan []byte
}

func NewSDR() *SDR {
	return &SDR{
		centerFreq:      137900000,
		sampleRate:      2080000,
		gain:            144,
		autoGain:        false,
		biasTee:         true,
		simulationMode:  false,
		dataSubscribers: make([]chan []byte, 0),
	}
}

func GetDeviceInfo() DeviceInfo {
	count := rtl.GetDeviceCount()
	if count > 0 {
		return DeviceInfo{
			Count:     count,
			Name:      fmt.Sprintf("RTL-SDR Device #0 (%d found)", count),
			IsPresent: true,
		}
	}
	return DeviceInfo{
		Count:     0,
		Name:      "No Hardware (Simulation Mode)",
		IsPresent: false,
	}
}

func (s *SDR) Subscribe() chan []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan []byte, 512)
	s.dataSubscribers = append(s.dataSubscribers, ch)
	return ch
}

func (s *SDR) Unsubscribe(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.dataSubscribers {
		if sub == ch {
			s.dataSubscribers = append(s.dataSubscribers[:i], s.dataSubscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

func (s *SDR) StartStream(freq, sampleRate, gain int, autoGain, biasTee, forceSim bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isStreaming {
		return nil
	}

	s.centerFreq = freq
	s.sampleRate = sampleRate
	s.gain = gain
	s.autoGain = autoGain
	s.biasTee = biasTee

	devCount := rtl.GetDeviceCount()
	if devCount == 0 || forceSim {
		s.simulationMode = true
		logger.Warnf("SDR", "Starting stream in SIMULATION mode (devCount=%d, forceSim=%v, freq=%.4fMHz)", devCount, forceSim, float64(freq)/1e6)
	} else {
		dev, err := rtl.Open(0)
		if err != nil {
			s.simulationMode = true
			logger.Errorf("SDR", "Failed to open RTL-SDR device #0: %v. Falling back to SIMULATION mode.", err)
		} else {
			s.device = dev
			s.simulationMode = false
			_ = dev.SetCenterFreq(freq)
			_ = dev.SetSampleRate(sampleRate)
			_ = dev.SetTunerGainMode(!autoGain)
			_ = dev.SetAgcMode(autoGain)
			if !autoGain {
				_ = dev.SetTunerGain(gain)
			}

			// Configure Bias-Tee (4.5V coaxial power for active LNA/antennas)
			if err := dev.SetBiasTee(biasTee); err != nil {
				logger.Warnf("SDR", "Failed to configure Bias-Tee (enable=%v): %v", biasTee, err)
			} else {
				logger.Infof("SDR", "Bias-Tee (4.5V coaxial power) configured: %v", biasTee)
			}

			_ = dev.ResetBuffer()
			logger.Infof("SDR", "Hardware stream started: freq=%.4fMHz, sample_rate=%dHz, tuner_gain=%.1fdB, auto_gain=%v, bias_tee=%v", float64(freq)/1e6, sampleRate, float64(gain)/10.0, autoGain, biasTee)
		}
	}

	s.isStreaming = true
	s.stopChan = make(chan struct{})

	if s.simulationMode {
		go s.runSimulationLoop()
	} else {
		go s.runHardwareLoop()
	}

	return nil
}

func (s *SDR) SetCenterFreq(freq int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.centerFreq == freq && s.device != nil {
		return nil
	}

	s.centerFreq = freq

	if s.device != nil && s.isStreaming && !s.simulationMode {
		err := s.device.SetCenterFreq(freq)
		if err != nil {
			logger.Errorf("SDR", "Failed to set center frequency to %d Hz: %v", freq, err)
			return err
		}
		logger.Infof("SDR", "Center frequency updated dynamically: %.4f MHz", float64(freq)/1e6)
	}
	return nil
}

func (s *SDR) SetGain(gain int, autoGain bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.gain == gain && s.autoGain == autoGain && s.device != nil {
		return nil
	}

	s.gain = gain
	s.autoGain = autoGain

	if s.device != nil && s.isStreaming && !s.simulationMode {
		_ = s.device.SetTunerGainMode(!autoGain)
		_ = s.device.SetAgcMode(autoGain)
		if !autoGain {
			_ = s.device.SetTunerGain(gain)
		}
		logger.Infof("SDR", "Tuner gain updated dynamically: gain=%.1fdB, auto=%v", float64(gain)/10.0, autoGain)
	}
	return nil
}

func (s *SDR) SetBiasTee(enable bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.biasTee = enable

	if s.device != nil && s.isStreaming && !s.simulationMode {
		err := s.device.SetBiasTee(enable)
		if err != nil {
			logger.Warnf("SDR", "Failed to update Bias-Tee to %v: %v", enable, err)
			return err
		}
		logger.Infof("SDR", "Bias-Tee updated dynamically: %v", enable)
	}
	return nil
}

func (s *SDR) StopStream() {
	s.mu.Lock()
	if !s.isStreaming {
		s.mu.Unlock()
		return
	}
	s.isStreaming = false
	close(s.stopChan)
	if s.device != nil {
		_ = s.device.Close()
		s.device = nil
	}
	for _, ch := range s.dataSubscribers {
		close(ch)
	}
	s.dataSubscribers = make([]chan []byte, 0)
	s.mu.Unlock()
	logger.Infof("SDR", "SDR stream stopped")
}

func (s *SDR) IsStreaming() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isStreaming
}

func (s *SDR) IsSimulation() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.simulationMode
}

func (s *SDR) runHardwareLoop() {
	bufSize := 1024 * 128
	buf := make([]byte, bufSize)
	lastErrLog := time.Now().Add(-10 * time.Second)

	for {
		select {
		case <-s.stopChan:
			return
		default:
			s.mu.Lock()
			dev := s.device
			s.mu.Unlock()

			if dev == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			n, err := dev.ReadSync(buf, len(buf))
			if err != nil || n == 0 {
				if time.Since(lastErrLog) >= 2*time.Second {
					logger.Warnf("SDR", "ReadSync warning (n=%d, err=%v), retrying...", n, err)
					lastErrLog = time.Now()
				}
				time.Sleep(10 * time.Millisecond)
				continue
			}

			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			s.broadcast(chunk)
		}
	}
}

func (s *SDR) runSimulationLoop() {
	bufSize := 1024 * 128
	buf := make([]byte, bufSize)
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()

	t := 0.0
	subcarrierFreq := 2400.0 // APT 2400Hz AM subcarrier
	sstvSubPhase := 0.0      // SSTV FM audio subcarrier phase
	phase := 0.0

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.mu.Lock()
			currentGain := s.gain
			currentAutoGain := s.autoGain
			currentFreq := s.centerFreq
			s.mu.Unlock()

			// Scale signal and noise by gain factor (1.0 at 40.0 dB / 400)
			gainScale := 1.0
			if !currentAutoGain {
				gainScale = float64(currentGain) / 400.0
				if gainScale < 0.05 {
					gainScale = 0.05
				}
			}

			isSSTV := currentFreq >= 144000000

			for i := 0; i < bufSize/2; i++ {
				var sub float64
				if isSSTV {
					// Simulate Robot 36 color SSTV (150ms per line, 320x240)
					linePeriod := 0.150
					lineNum := int(t / linePeriod)
					lineTime := math.Mod(t, linePeriod)
					ny := math.Mod(float64(lineNum)/60.0, 1.0)
					isEven := (lineNum % 2) == 0

					var toneFreq float64
					if lineTime < 0.009 {
						toneFreq = 1200.0 // 9ms Horizontal Sync Pulse
					} else if lineTime < 0.012 {
						toneFreq = 1500.0 // 3ms Porch
					} else if lineTime < 0.100 {
						// 88ms Luminance (Y) scan
						nx := (lineTime - 0.012) / 0.088
						r, g, b := simSSTVSceneRGB(nx, ny)
						yVal := 0.299*r + 0.587*g + 0.114*b
						toneFreq = 1500.0 + yVal*800.0
					} else if lineTime < 0.1045 {
						if isEven {
							toneFreq = 1500.0
						} else {
							toneFreq = 2300.0
						}
					} else if lineTime < 0.106 {
						toneFreq = 1900.0
					} else {
						// 44ms Chrominance (R-Y on even lines, B-Y on odd lines)
						nx := (lineTime - 0.106) / 0.044
						r, g, b := simSSTVSceneRGB(nx, ny)
						yVal := 0.299*r + 0.587*g + 0.114*b
						var cNorm float64
						if isEven {
							cr := 0.5 + (r-yVal)/1.402
							cNorm = math.Max(0, math.Min(1, cr))
						} else {
							cb := 0.5 + (b-yVal)/1.772
							cNorm = math.Max(0, math.Min(1, cb))
						}
						toneFreq = 1500.0 + cNorm*800.0
					}

					sstvSubPhase += 2.0 * math.Pi * toneFreq / float64(s.sampleRate)
					if sstvSubPhase > 2.0*math.Pi {
						sstvSubPhase -= 2.0 * math.Pi
					}
					sub = 0.85 * math.Sin(sstvSubPhase)
				} else {
					// 0.5 sec line period (2 lines per second, 2080 words per line)
					lineNum := int(t * 2.0)
					linePhase := math.Mod(t*2.0, 1.0)
					wordIdx := int(linePhase * 2080.0)
					var videoVal float64

					if wordIdx < 39 {
						// Sync A: 7 cycles of 1040 Hz square wave (wordRate=4160Hz -> 4 words per cycle, 28 words active)
						if wordIdx >= 4 && wordIdx < 32 && ((wordIdx-4)%4 < 2) {
							videoVal = 0.95
						} else {
							videoVal = 0.05
						}
					} else if wordIdx < 86 {
						// Space A + minute marker every 120 lines
						if lineNum%120 < 4 {
							videoVal = 0.90
						} else {
							videoVal = 0.08
						}
					} else if wordIdx < 995 {
						// Channel A (Visible): Simulated cloud vortices, frontal band, and landmass
						nx := float64(wordIdx-86) / 909.0
						ny := float64(lineNum) / 120.0
						dx := nx - 0.52
						dy := math.Mod(ny, 1.5) - 0.75
						dist := math.Sqrt(dx*dx + dy*dy)
						angle := math.Atan2(dy, dx)
						spiral := math.Sin(angle*2.0 - dist*18.0 + ny*2.0)
						swirl := math.Exp(-dist*3.8) * (0.55 + 0.45*spiral)
						front := math.Exp(-math.Pow((nx-0.25-0.35*math.Sin(ny*1.8))*5.0, 2)) * (0.6 + 0.3*math.Cos(nx*25.0+ny*8.0))
						land := 0.15 + 0.12*math.Sin(nx*7.0)*math.Cos(ny*4.5)
						videoVal = math.Max(0.08, math.Min(0.95, land+swirl*0.65+front*0.45))
					} else if wordIdx < 1040 {
						// Telemetry A: 16-step grayscale calibration wedge (8 lines per wedge step)
						step := (lineNum / 8) % 16
						videoVal = 0.08 + 0.85*(float64(step)/15.0)
					} else if wordIdx < 1079 {
						// Sync B: 7 pulses at 832 Hz (5 words per cycle, 35 words active)
						wb := wordIdx - 1040
						if wb >= 2 && wb < 37 && ((wb-2)%5 < 3) {
							videoVal = 0.95
						} else {
							videoVal = 0.05
						}
					} else if wordIdx < 1126 {
						// Space B
						if lineNum%120 < 4 {
							videoVal = 0.08
						} else {
							videoVal = 0.88
						}
					} else if wordIdx < 2035 {
						// Channel B (Thermal Infrared): Cloud top temperatures + sea/land thermal gradient
						nx := float64(wordIdx-1126) / 909.0
						ny := float64(lineNum) / 120.0
						dx := nx - 0.52
						dy := math.Mod(ny, 1.5) - 0.75
						dist := math.Sqrt(dx*dx + dy*dy)
						angle := math.Atan2(dy, dx)
						spiral := math.Sin(angle*2.0 - dist*18.0 + ny*2.0)
						swirl := math.Exp(-dist*3.2) * (0.65 + 0.35*spiral)
						front := math.Exp(-math.Pow((nx-0.25-0.35*math.Sin(ny*1.8))*4.2, 2)) * 0.75
						thermalGrad := 0.22 + 0.15*math.Cos(ny*1.2) - 0.08*math.Sin(nx*5.0)
						videoVal = math.Max(0.10, math.Min(0.96, thermalGrad+swirl*0.70+front*0.50))
					} else {
						// Telemetry B: 16-step grayscale wedge
						step := (lineNum / 8) % 16
						videoVal = 0.08 + 0.85*(float64(step)/15.0)
					}

					// AM modulation on 2400Hz subcarrier
					sub = videoVal * math.Sin(2*math.Pi*subcarrierFreq*t)
				}

				// FM modulation: instantaneous frequency deviation is proportional to subcarrier audio
				maxDev := 17000.0 // +/- 17 kHz FM deviation
				phase += 2.0 * math.Pi * (maxDev * sub) / float64(s.sampleRate)
				if phase > 2.0*math.Pi {
					phase -= 2.0 * math.Pi
				} else if phase < -2.0*math.Pi {
					phase += 2.0 * math.Pi
				}

				noiseI := (rand.Float64() - 0.5) * 0.04 * gainScale
				noiseQ := (rand.Float64() - 0.5) * 0.04 * gainScale

				valI := (math.Cos(phase) * gainScale) + noiseI
				valQ := (math.Sin(phase) * gainScale) + noiseQ

				buf[2*i] = byte(math.Max(0, math.Min(255, (valI+1.0)*127.5)))
				buf[2*i+1] = byte(math.Max(0, math.Min(255, (valQ+1.0)*127.5)))

				t += 1.0 / float64(s.sampleRate)
			}

			chunk := make([]byte, bufSize)
			copy(chunk, buf)
			s.broadcast(chunk)
		}
	}
}

// simSSTVSceneRGB generates a synthetic color space-station & Earth horizon test card for SSTV simulation
func simSSTVSceneRGB(nx, ny float64) (r, g, b float64) {
	// Top 18%: SMPTE-style color bar header
	if ny < 0.18 {
		bar := int(nx * 7.0)
		switch bar {
		case 0:
			return 0.92, 0.92, 0.92 // White
		case 1:
			return 0.92, 0.92, 0.10 // Yellow
		case 2:
			return 0.10, 0.92, 0.92 // Cyan
		case 3:
			return 0.10, 0.90, 0.20 // Green
		case 4:
			return 0.92, 0.15, 0.85 // Magenta
		case 5:
			return 0.92, 0.15, 0.15 // Red
		default:
			return 0.15, 0.25, 0.95 // Blue
		}
	}

	// Curved blue Earth limb at bottom + deep space + golden solar panels
	earthCenterY := 1.45
	dx := nx - 0.5
	dy := ny - earthCenterY
	distEarth := math.Sqrt(dx*dx + dy*dy)

	if distEarth < 0.82 {
		// Blue ocean + white cloud swirls
		cloud := 0.5 + 0.5*math.Sin(nx*18.0+math.Cos(ny*12.0)*2.5)*math.Cos(ny*15.0)
		r = 0.08 + 0.75*math.Pow(cloud, 2.2)
		g = 0.35 + 0.55*math.Pow(cloud, 1.8)
		b = 0.82 + 0.16*cloud
		return
	} else if distEarth < 0.86 {
		// Cyan atmospheric glow
		glow := 1.0 - (distEarth-0.82)/0.04
		return 0.15 * glow, 0.65 * glow, 0.98 * glow
	}

	// Space background with ISS solar array silhouette
	if math.Abs(ny-0.42) < 0.14 && ((nx > 0.18 && nx < 0.34) || (nx > 0.66 && nx < 0.82)) {
		// Golden solar panels
		return 0.85, 0.62, 0.12
	}
	if math.Abs(ny-0.42) < 0.04 && nx >= 0.30 && nx <= 0.70 {
		// Metallic truss / module
		return 0.82, 0.85, 0.90
	}
	return 0.04, 0.05, 0.12
}

func (s *SDR) broadcast(buf []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.dataSubscribers {
		select {
		case sub <- buf:
		default:
			logger.Warnf("SDR", "Subscriber queue full! Dropped %d bytes of IQ data", len(buf))
		}
	}
}
