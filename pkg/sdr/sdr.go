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
	mu             sync.Mutex
	device         *rtl.Context
	centerFreq     int
	sampleRate     int
	gain           int
	autoGain       bool
	isStreaming    bool
	simulationMode bool
	stopChan       chan struct{}
	dataSubscribers []chan []byte
}

func NewSDR() *SDR {
	return &SDR{
		centerFreq:      137100000,
		sampleRate:      2080000,
		gain:            400,
		autoGain:        false,
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

func (s *SDR) StartStream(freq, sampleRate, gain int, autoGain, forceSim bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isStreaming {
		return nil
	}

	s.centerFreq = freq
	s.sampleRate = sampleRate
	s.gain = gain
	s.autoGain = autoGain

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
			dev.SetCenterFreq(freq)
			dev.SetSampleRate(sampleRate)
			dev.SetTunerGainMode(!autoGain)
			if !autoGain {
				dev.SetTunerGain(gain)
			}
			_ = dev.SetOffsetTuning(true)
			dev.ResetBuffer()
			logger.Infof("SDR", "Hardware stream started: freq=%.4fMHz, sample_rate=%dHz, tuner_gain=%.1fdB, auto_gain=%v, offset_tuning=true", float64(freq)/1e6, sampleRate, float64(gain)/10.0, autoGain)
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

func (s *SDR) SetGain(gain int, autoGain bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gain = gain
	s.autoGain = autoGain

	if s.device != nil && s.isStreaming && !s.simulationMode {
		_ = s.device.SetTunerGainMode(!autoGain)
		if !autoGain {
			_ = s.device.SetTunerGain(gain)
		}
		logger.Infof("SDR", "Tuner gain updated dynamically: gain=%.1fdB, auto=%v", float64(gain)/10.0, autoGain)
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

	for {
		select {
		case <-s.stopChan:
			return
		default:
			if s.device == nil {
				return
			}
			n, err := s.device.ReadSync(buf, len(buf))
			if err != nil || n == 0 {
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
	phase := 0.0

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.mu.Lock()
			currentGain := s.gain
			currentAutoGain := s.autoGain
			s.mu.Unlock()

			// Scale signal and noise by gain factor (1.0 at 40.0 dB / 400)
			gainScale := 1.0
			if !currentAutoGain {
				gainScale = float64(currentGain) / 400.0
				if gainScale < 0.05 {
					gainScale = 0.05
				}
			}

			// Synthesize simulated APT IQ signal with noise and subcarrier
			for i := 0; i < bufSize/2; i++ {
				// 0.5 sec line pulse simulation (APT sync frame emulation)
				linePhase := math.Mod(t*2.0, 1.0) // 2 lines per second
				syncPulse := 0.2
				if linePhase < 0.05 {
					syncPulse = 0.9 // Sync A high pulse
				} else if linePhase > 0.5 && linePhase < 0.55 {
					syncPulse = 0.9 // Sync B high pulse
				}

				// AM subcarrier 2400Hz audio signal m(t)
				sub := (0.2 + 0.8*syncPulse*math.Sin(2*math.Pi*subcarrierFreq*t))

				// WFM modulation: instantaneous frequency deviation is proportional to subcarrier audio
				maxDev := 17000.0 // +/- 17 kHz WFM deviation for NOAA APT
				phase += 2.0 * math.Pi * (maxDev * sub) / float64(s.sampleRate)
				if phase > 2.0*math.Pi {
					phase -= 2.0 * math.Pi
				}

				noiseI := (rand.Float64() - 0.5) * 0.08 * gainScale
				noiseQ := (rand.Float64() - 0.5) * 0.08 * gainScale

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
