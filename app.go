package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"twnoaa/pkg/ai"
	"twnoaa/pkg/datastore"
	"twnoaa/pkg/decoder"
	"twnoaa/pkg/dsp"
	"twnoaa/pkg/logger"
	"twnoaa/pkg/sdr"
	"twnoaa/pkg/tracker"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx            context.Context
	store          *datastore.Store
	tracker        *tracker.Tracker
	sdrService     *sdr.SDR
	recording      bool
	recMu          sync.Mutex
	recordedPasses map[string]time.Time
	recPassMu      sync.Mutex
}

func NewApp() *App {
	_ = logger.Init("")
	st, err := datastore.NewStore()
	if err != nil {
		logger.Errorf("APP", "Failed to open datastore: %v", err)
	}

	tr := tracker.NewTracker()
	sd := sdr.NewSDR()

	return &App{
		store:          st,
		tracker:        tr,
		sdrService:     sd,
		recordedPasses: make(map[string]time.Time),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.Infof("APP", "twnoaa application started")

	// Pre-fetch TLEs
	go func() {
		err := a.tracker.FetchTLEs()
		if err != nil {
			logger.Warnf("TRACKER", "TLE fetch note: %v", err)
		} else {
			logger.Infof("TRACKER", "Satellite TLE data updated successfully")
		}
	}()

	// Start Auto-recording scheduler loop
	go a.startAutoRecordScheduler()
}

func (a *App) shutdown(ctx context.Context) {
	logger.Infof("APP", "Application shutting down")
	a.sdrService.StopStream()
	if a.store != nil {
		_ = a.store.Close()
	}
	logger.Close()
}

func (a *App) startAutoRecordScheduler() {
	logger.Infof("SCHEDULER", "Auto-record scheduler loop started (interval=10s)")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			logger.Infof("SCHEDULER", "Auto-record scheduler stopped")
			return
		case <-ticker.C:
			cfg := a.store.GetConfig()
			if !cfg.AutoRecord {
				continue
			}

			a.recMu.Lock()
			isRec := a.recording
			a.recMu.Unlock()

			if isRec {
				continue
			}

			states, err := a.tracker.GetCurrentStates(cfg.ObserverLat, cfg.ObserverLon, cfg.ObserverAlt)
			if err != nil {
				logger.Errorf("SCHEDULER", "Failed to get satellite states: %v", err)
				continue
			}

			now := time.Now()
			for _, sat := range states {
				// Auto-record scheduler prioritizes 137 MHz weather satellites when Bias-Tee (137 MHz SAW LNA) is enabled,
				// because the 137 MHz SAW filter blocks 145.8 MHz SSTV unless Bias-Tee / LNA is bypassed.
				if cfg.BiasTee && sat.Frequency > 140000000 {
					continue
				}
				if sat.IsActive { // Elevation >= 5.0°
					passKey := fmt.Sprintf("%s_%s", sat.Name, now.Format("20060102_15"))

					a.recPassMu.Lock()
					_, alreadyRecorded := a.recordedPasses[passKey]
					a.recPassMu.Unlock()

					if alreadyRecorded {
						logger.Debugf("SCHEDULER", "Pass already recorded for key %s, skipping", passKey)
						continue
					}

					remainingSec := a.tracker.GetRemainingPassDuration(sat.Name, cfg.ObserverLat, cfg.ObserverLon, cfg.ObserverAlt, 5.0)
					const minPassThresholdSec = 180.0 // Minimum 3 minutes required

					if remainingSec < minPassThresholdSec {
						logger.Infof("SCHEDULER", "Skipping auto-recording for %s: remaining duration (%.0fs) is less than minimum required (%.0fs)",
							sat.Name, remainingSec, minPassThresholdSec)
						a.recPassMu.Lock()
						a.recordedPasses[passKey] = now
						a.recPassMu.Unlock()
						continue
					}

					a.recPassMu.Lock()
					a.recordedPasses[passKey] = now
					a.recPassMu.Unlock()

					duration := cfg.AutoRecordDuration
					if duration <= 0 {
						duration = 600
					}
					// Cap duration to the remaining pass time so we don't record noise after LOS
					if int(remainingSec) < duration {
						duration = int(remainingSec)
					}

					logger.Infof("SCHEDULER", "Active pass detected! Sat: %s (Freq: %.4fMHz, Elev: %.1f°, Az: %.1f°, Remaining: %.0fs). Starting auto-recording for %ds",
						sat.Name, float64(sat.Frequency)/1e6, sat.Elevation, sat.Azimuth, remainingSec, duration)
					if a.ctx != nil {
						runtime.EventsEmit(a.ctx, "auto_record_started", map[string]interface{}{
							"sat_name":  sat.Name,
							"frequency": sat.Frequency,
							"duration":  duration,
						})
					}
					a.StartRecordingAndDecode(sat.Name, sat.Frequency, duration, cfg.TunerGain, cfg.TunerGainAuto)
					break
				}
			}
		}
	}
}

// Config API
func (a *App) GetConfig() datastore.Config {
	return a.store.GetConfig()
}

func (a *App) SaveConfig(cfg datastore.Config) string {
	if err := a.store.UpdateConfig(cfg); err != nil {
		return fmt.Sprintf("Error saving config: %v", err)
	}
	if a.sdrService != nil {
		_ = a.sdrService.SetBiasTee(cfg.BiasTee)
	}
	return "Config saved successfully"
}

func (a *App) FetchIPLocation() (*tracker.IPLocation, error) {
	loc, err := tracker.GetIPLocation()
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch location: %w", err)
	}
	return loc, nil
}

// Pass Tracker API
func (a *App) GetSatelliteStates() ([]*tracker.SatelliteState, error) {
	cfg := a.store.GetConfig()
	return a.tracker.GetCurrentStates(cfg.ObserverLat, cfg.ObserverLon, cfg.ObserverAlt)
}

func (a *App) GetUpcomingPasses(hours int) ([]*tracker.PassDetail, error) {
	if hours <= 0 {
		hours = 24
	}
	cfg := a.store.GetConfig()
	return a.tracker.GetUpcomingPasses(cfg.ObserverLat, cfg.ObserverLon, cfg.ObserverAlt, hours)
}

func (a *App) RefreshTLEs() string {
	err := a.tracker.FetchTLEs()
	if err != nil {
		return fmt.Sprintf("TLE refresh note: %v", err)
	}
	return "TLE updated successfully"
}

// SDR Receiver API
func (a *App) GetSDRDeviceInfo() sdr.DeviceInfo {
	return sdr.GetDeviceInfo()
}

func (a *App) StartSDR(freq, sampleRate, gain int, autoGain, forceSim bool) string {
	cfg := a.store.GetConfig()
	err := a.sdrService.StartStream(freq, sampleRate, gain, autoGain, cfg.BiasTee, forceSim)
	if err != nil {
		return fmt.Sprintf("Failed to start SDR: %v", err)
	}

	// Stream waterfall data to frontend with throttling and skipping during recording
	go func() {
		sub := a.sdrService.Subscribe()
		defer a.sdrService.Unsubscribe(sub)

		lastEmitTime := time.Now()
		const spectrumInterval = 100 * time.Millisecond // Limit spectrum updates to max 10 FPS

		for buf := range sub {
			// Skip spectrum calculation and emitting while recording (auto or manual)
			a.recMu.Lock()
			isRec := a.recording
			a.recMu.Unlock()

			if isRec {
				continue
			}

			if a.ctx != nil && time.Since(lastEmitTime) >= spectrumInterval {
				lastEmitTime = time.Now()
				spectrum := dsp.CalculatePowerSpectrum(buf, 128)
				runtime.EventsEmit(a.ctx, "sdr_spectrum", spectrum)
			}
		}
	}()

	return "SDR stream started"
}

func (a *App) SetSDRCenterFreq(freq int) string {
	err := a.sdrService.SetCenterFreq(freq)
	if err != nil {
		return fmt.Sprintf("Failed to set SDR frequency: %v", err)
	}
	return "SDR frequency updated"
}

func (a *App) SetSDRGain(gain int, autoGain bool) string {
	err := a.sdrService.SetGain(gain, autoGain)
	if err != nil {
		return fmt.Sprintf("Failed to set SDR gain: %v", err)
	}
	return "SDR gain updated"
}

func (a *App) SetSDRBiasTee(enable bool) string {
	err := a.sdrService.SetBiasTee(enable)
	if err != nil {
		return fmt.Sprintf("Failed to set SDR Bias-Tee: %v", err)
	}
	cfg := a.store.GetConfig()
	cfg.BiasTee = enable
	_ = a.store.UpdateConfig(cfg)
	return "SDR Bias-Tee updated"
}

func (a *App) StopSDR() string {
	a.sdrService.StopStream()
	return "SDR stream stopped"
}

// Recording & Decoder API
func (a *App) IsRecording() bool {
	a.recMu.Lock()
	defer a.recMu.Unlock()
	return a.recording
}

func (a *App) StartRecordingAndDecode(satName string, freq int, durationSec int, gain int, autoGain bool) error {
	a.recMu.Lock()
	if a.recording {
		a.recMu.Unlock()
		logger.Warnf("RECORDING", "Recording request ignored: recording already in progress")
		return fmt.Errorf("recording is already in progress")
	}
	a.recording = true
	a.recMu.Unlock()

	go func() {
		cfg := a.store.GetConfig()
		wasStreaming := a.sdrService.IsStreaming()

		defer func() {
			a.recMu.Lock()
			a.recording = false
			a.recMu.Unlock()

			// If SDR was not streaming before recording, stop it now
			if !wasStreaming {
				a.sdrService.StopStream()
			}

			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "auto_record_ended", map[string]interface{}{
					"sat_name": satName,
				})
			}
		}()

		if !wasStreaming {
			_ = a.sdrService.StartStream(freq, cfg.SampleRate, gain, autoGain, cfg.BiasTee, cfg.SimulationMode)
		} else {
			_ = a.sdrService.SetCenterFreq(freq)
			_ = a.sdrService.SetGain(gain, autoGain)
		}

		sub := a.sdrService.Subscribe()
		defer a.sdrService.Unsubscribe(sub)

		dspEngine := dsp.NewDSP(cfg.SampleRate, 20800)
		var pcmSamples []float64

		gainDisplay := fmt.Sprintf("%.1fdB", float64(gain)/10.0)
		if autoGain {
			gainDisplay = "AUTO (AGC)"
		}
		logger.Infof("RECORDING", "Recording started: sat=%s, freq=%.4fMHz, duration=%ds, sample_rate=%dHz, tuner_gain=%s, save_raw_iq=%v", satName, float64(freq)/1e6, durationSec, cfg.SampleRate, gainDisplay, cfg.SaveRawIQ)

		// Raw IQ file stream writer (always enabled for METEOR-M LRPT digital satellites or when SaveRawIQ is checked)
		isLRPTSat := strings.HasPrefix(strings.ToUpper(satName), "METEOR") && !a.sdrService.IsSimulation()
		saveRaw := cfg.SaveRawIQ || isLRPTSat
		baseName := fmt.Sprintf("%s_%s", satName, time.Now().Format("20060102_150405"))
		rawIQDir := cfg.DataDir + "/raw_iq"
		rawIQPath := ""
		var rawIQFile *os.File
		if saveRaw {
			_ = os.MkdirAll(rawIQDir, 0755)
			rawIQPath = fmt.Sprintf("%s/%s_raw.iq", rawIQDir, baseName)
			f, err := os.Create(rawIQPath)
			if err != nil {
				logger.Warnf("RECORDING", "Failed to create raw IQ file %s: %v", rawIQPath, err)
				rawIQPath = ""
			} else {
				rawIQFile = f
				defer rawIQFile.Close()
			}
		}

		var totalRssi float64
		var rssiCount int
		peakRssi := -100.0
		minRssi := 0.0

		var signalHistory []datastore.SignalPoint
		lastSampleTime := time.Now()

		startTime := time.Now()
		lastLogTime := time.Now()
		timeout := time.After(time.Duration(durationSec) * time.Second)

		for {
			select {
			case <-timeout:
				goto DONE
			case buf, ok := <-sub:
				if !ok {
					goto DONE
				}
				if rawIQFile != nil {
					_, _ = rawIQFile.Write(buf)
				}

				rssi := dsp.CalculateRSSI(buf)
				totalRssi += rssi
				rssiCount++
				if rssi > peakRssi {
					peakRssi = rssi
				}
				if minRssi == 0.0 || rssi < minRssi {
					minRssi = rssi
				}

				// Sample signal and noise every 0.5 seconds
				if time.Since(lastSampleTime) >= 500*time.Millisecond {
					lastSampleTime = time.Now()
					sig, noise, snr := dsp.CalculateSignalAndNoise(buf)
					elapsedSec := time.Since(startTime).Seconds()
					signalHistory = append(signalHistory, datastore.SignalPoint{
						TimeSec: math.Round(elapsedSec*10) / 10,
						Signal:  math.Round(sig*10) / 10,
						Noise:   math.Round(noise*10) / 10,
						SNR:     math.Round(snr*10) / 10,
					})
				}

				if time.Since(lastLogTime) >= 60*time.Second {
					elapsed := int(time.Since(startTime).Seconds())
					logger.Infof("RECORDING", "[%s] Progress: %ds / %ds | Current RSSI: %.1f dBFS | Peak RSSI: %.1f dBFS", satName, elapsed, durationSec, rssi, peakRssi)
					lastLogTime = time.Now()
				}

				iqComplex := dsp.ConvertIQToComplex(buf)
				audio := dspEngine.DemodulateFM(iqComplex)
				pcmSamples = append(pcmSamples, audio...)
			}
		}

	DONE:
		a.sdrService.Unsubscribe(sub)
		if rawIQFile != nil {
			_ = rawIQFile.Close()
			rawIQFile = nil
		}
		a.recMu.Lock()
		a.recording = false
		a.recMu.Unlock()

		avgRssi := -100.0
		if rssiCount > 0 {
			avgRssi = totalRssi / float64(rssiCount)
		}
		logger.Infof("RECORDING", "Recording finished for %s: duration=%.1fs, pcm_samples=%d, avg_rssi=%.1f dBFS, peak_rssi=%.1f dBFS, min_rssi=%.1f dBFS, signal_samples=%d, raw_iq=%s", satName, time.Since(startTime).Seconds(), len(pcmSamples), avgRssi, peakRssi, minRssi, len(signalHistory), rawIQPath)

		var res *decoder.DecodeResult
		var err error
		upperSat := strings.ToUpper(satName)
		isSSTVSat := freq >= 144000000 || strings.Contains(upperSat, "ISS") || strings.Contains(upperSat, "SONATE") || strings.Contains(upperSat, "SSTV")
		if isSSTVSat {
			res, err = decoder.DecodeSSTV(pcmSamples, 20800, cfg.DataDir+"/images", baseName)
		} else if isLRPTSat && rawIQPath != "" {
			res, err = decoder.DecodeLRPT(rawIQPath, cfg.SampleRate, cfg.DataDir+"/images", baseName)
		} else {
			res, err = decoder.DecodeAPT(pcmSamples, 20800, cfg.DataDir+"/images", baseName)
		}
		if err != nil {
			logger.Errorf("RECORDING", "Decode failed for %s: %v", satName, err)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "decode_error", err.Error())
			}
			return
		}

		// Save intermediate demodulated audio WAV file
		audioDir := cfg.DataDir + "/audio"
		_ = os.MkdirAll(audioDir, 0755)
		wavPath := fmt.Sprintf("%s/%s_audio.wav", audioDir, baseName)
		int16Audio := dsp.ConvertFloatToINT16(pcmSamples)
		if err := dsp.SaveWAV(wavPath, 20800, int16Audio); err != nil {
			logger.Warnf("RECORDING", "Failed to save intermediate audio WAV file: %v", err)
			wavPath = ""
		} else {
			logger.Infof("RECORDING", "Intermediate audio WAV saved successfully: %s", wavPath)
		}

		maxElev, maxElevAz := a.getSatMaxElevationInfo(satName, cfg, startTime)

		// Create record
		record := &datastore.PassRecord{
			ID:             baseName,
			SatName:        satName,
			Frequency:      freq,
			Timestamp:      time.Now(),
			ObserverLat:    cfg.ObserverLat,
			ObserverLon:    cfg.ObserverLon,
			MaxElevation:   maxElev,
			MaxElevationAz: maxElevAz,
			AvgRSSI:        avgRssi,
			PeakRSSI:       peakRssi,
			Gain:           gain,
			GainAuto:       autoGain,
			ImagePath:      res.FullImagePath,
			ChannelA:       res.ChanAPath,
			ChannelB:       res.ChanBPath,
			AudioPath:      wavPath,
			RawIQPath:      rawIQPath,
			SignalHistory:  signalHistory,
		}

		_ = a.store.AddRecord(record)
		logger.Infof("RECORDING", "Pass record saved successfully: ID=%s, MaxElev=%.1f°, MaxElevAz=%.1f°, Image=%s", record.ID, record.MaxElevation, record.MaxElevationAz, record.ImagePath)

		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "decode_complete", record)
		}

		// Auto AI analysis if enabled
		if cfg.AutoAnalyze {
			logger.Infof("RECORDING", "Triggering auto AI analysis for record %s", record.ID)
			a.RunAIAnalysis(record.ID, "ja")
		}
	}()

	return nil
}

func (a *App) getSatMaxElevationInfo(satName string, cfg datastore.Config, recStartTime time.Time) (float64, float64) {
	// Search in a window from 20 minutes before recStartTime to 40 minutes after
	passes, err := a.tracker.GetPassesInWindow(cfg.ObserverLat, cfg.ObserverLon, cfg.ObserverAlt, recStartTime.Add(-20*time.Minute), recStartTime.Add(40*time.Minute))
	if err == nil {
		for _, p := range passes {
			if p.SatName == satName {
				return p.MaxElev, p.MaxElevAz
			}
		}
	}
	states, err := a.tracker.GetCurrentStates(cfg.ObserverLat, cfg.ObserverLon, cfg.ObserverAlt)
	if err == nil {
		for _, s := range states {
			if s.Name == satName && s.Elevation > 0 {
				return s.Elevation, s.Azimuth
			}
		}
	}
	return 0.0, 0.0
}

func (a *App) GetPassRecords() []*datastore.PassRecord {
	return a.store.GetRecords()
}

func (a *App) RunAIAnalysis(recordID string, lang string) string {
	records := a.store.GetRecords()
	var target *datastore.PassRecord
	for _, r := range records {
		if r.ID == recordID {
			target = r
			break
		}
	}

	if target == nil {
		return "Error: Record not found"
	}

	if target.ImagePath == "" {
		return "Error: No image path for this record"
	}

	if _, err := os.Stat(target.ImagePath); os.IsNotExist(err) {
		return "Error: Image file does not exist on disk"
	}

	if lang == "" {
		lang = "ja"
	}

	cfg := a.store.GetConfig()
	req := ai.AnalysisRequest{
		SatelliteName:  target.SatName,
		FrequencyMHz:   float64(target.Frequency) / 1e6,
		TimestampStr:   target.Timestamp.Format("2006-01-02 15:04:05 (JST)"),
		ObserverCity:   cfg.ObserverCity,
		ObserverLat:    target.ObserverLat,
		ObserverLon:    target.ObserverLon,
		MaxElevation:   target.MaxElevation,
		MaxElevationAz: target.MaxElevationAz,
		ImagePath:      filepath.Clean(target.ImagePath),
		Lang:           lang,
	}

	go func() {
		analysis, err := ai.AnalyzeSatelliteImage(a.ctx, cfg, req)
		if err != nil {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "ai_error", err.Error())
			}
			return
		}

		target.AIAnalysis = analysis
		target.AnalysisDate = time.Now()
		_ = a.store.UpdateRecord(target)

		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "ai_complete", target)
		}
	}()

	return "AI analysis started in background"
}

func (a *App) GetImageBase64(imagePath string) string {
	if imagePath == "" {
		return ""
	}
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func (a *App) GetAudioBase64(audioPath string) string {
	if audioPath == "" {
		return ""
	}
	data, err := os.ReadFile(audioPath)
	if err != nil {
		logger.Warnf("APP", "Failed to read audio file %s: %v", audioPath, err)
		return ""
	}
	return "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(data)
}

// SelectWAVFile opens a file dialog to select a WAV file and returns its path
func (a *App) SelectWAVFile() string {
	if a.ctx == nil {
		return ""
	}

	selectedPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select WAV Audio File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "WAV Audio (*.wav)",
				Pattern:     "*.wav",
			},
		},
	})
	if err != nil {
		logger.Warnf("APP", "Error selecting WAV file: %v", err)
		return ""
	}

	return selectedPath
}

func (a *App) DeletePassRecord(recordID string) string {
	records := a.store.GetRecords()
	for _, r := range records {
		if r.ID == recordID {
			if r.ImagePath != "" {
				_ = os.Remove(r.ImagePath)
			}
			if r.ChannelA != "" {
				_ = os.Remove(r.ChannelA)
			}
			if r.ChannelB != "" {
				_ = os.Remove(r.ChannelB)
			}
			if r.AudioPath != "" {
				_ = os.Remove(r.AudioPath)
			}
			break
		}
	}
	if err := a.store.DeleteRecord(recordID); err != nil {
		return fmt.Sprintf("Error deleting record: %v", err)
	}
	return "Record deleted successfully"
}

func (a *App) DeletePassRecords(recordIDs []string) string {
	if len(recordIDs) == 0 {
		return "No records specified"
	}
	idMap := make(map[string]bool, len(recordIDs))
	for _, id := range recordIDs {
		idMap[id] = true
	}

	records := a.store.GetRecords()
	for _, r := range records {
		if idMap[r.ID] {
			if r.ImagePath != "" {
				_ = os.Remove(r.ImagePath)
			}
			if r.ChannelA != "" {
				_ = os.Remove(r.ChannelA)
			}
			if r.ChannelB != "" {
				_ = os.Remove(r.ChannelB)
			}
			if r.AudioPath != "" {
				_ = os.Remove(r.AudioPath)
			}
		}
	}
	if err := a.store.DeleteRecords(recordIDs); err != nil {
		return fmt.Sprintf("Error deleting records: %v", err)
	}
	return "Records deleted successfully"
}


// SelectAndDecodeWAV opens a file dialog to select a WAV file and decode it
func (a *App) SelectAndDecodeWAV() string {
	if a.ctx == nil {
		return "Error: App context not initialized"
	}

	selectedPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select NOAA APT WAV Audio File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "WAV Audio (*.wav)",
				Pattern:     "*.wav",
			},
		},
	})
	if err != nil {
		return fmt.Sprintf("Error selecting file: %v", err)
	}

	if selectedPath == "" {
		return "No file selected"
	}

	return a.DecodeWAVFile(selectedPath)
}

// DecodeWAVFile decodes a specified WAV audio file and creates a PassRecord
func (a *App) DecodeWAVFile(wavPath string) string {
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		return fmt.Sprintf("Error: File %s does not exist", wavPath)
	}

	cfg := a.store.GetConfig()
	satName := "WAV_Test"

	base := filepath.Base(wavPath)
	ext := filepath.Ext(base)
	nameWithoutExt := base[:len(base)-len(ext)]
	if len(nameWithoutExt) > 0 {
		satName = fmt.Sprintf("WAV_%s", nameWithoutExt)
	}

	baseName := fmt.Sprintf("%s_%s", satName, time.Now().Format("20060102_150405"))
	outDir := cfg.DataDir + "/images"

	logger.Infof("APP", "Starting WAV test decode: file=%s, baseName=%s", wavPath, baseName)

	go func() {
		upperBase := strings.ToUpper(base)
		var res *decoder.DecodeResult
		var err error
		freq := 137100000
		if strings.Contains(upperBase, "ISS") || strings.Contains(upperBase, "SONATE") || strings.Contains(upperBase, "SSTV") || strings.Contains(upperBase, "PD120") || strings.Contains(upperBase, "ROBOT") || strings.Contains(upperBase, "MARTIN") {
			freq = 145800000
			res, err = decoder.DecodeSSTVWAVFile(wavPath, outDir, baseName)
		} else {
			res, err = decoder.DecodeWAVFile(wavPath, outDir, baseName)
		}
		if err != nil {
			logger.Errorf("APP", "WAV decode failed: %v", err)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "decode_error", err.Error())
			}
			return
		}

		record := &datastore.PassRecord{
			ID:             baseName,
			SatName:        satName,
			Frequency:      freq,
			Timestamp:      time.Now(),
			ObserverLat:    cfg.ObserverLat,
			ObserverLon:    cfg.ObserverLon,
			MaxElevation:   90.0,
			MaxElevationAz: 0.0,
			AvgRSSI:        0.0,
			PeakRSSI:       0.0,
			Gain:           cfg.TunerGain,
			GainAuto:       cfg.TunerGainAuto,
			ImagePath:      res.FullImagePath,
			ChannelA:       res.ChanAPath,
			ChannelB:       res.ChanBPath,
			AudioPath:      wavPath,
		}

		_ = a.store.AddRecord(record)
		logger.Infof("APP", "WAV decode completed and record saved: ID=%s, Image=%s", record.ID, record.ImagePath)

		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "decode_complete", record)
		}

		if cfg.AutoAnalyze {
			logger.Infof("APP", "Triggering auto AI analysis for WAV test record %s", record.ID)
			a.RunAIAnalysis(record.ID, "ja")
		}
	}()

	return fmt.Sprintf("WAV decode started for %s...", base)
}

// SelectAndDecodeRawIQ opens a file dialog to select a Raw IQ (.iq / .raw) file and demodulate/decode it
func (a *App) SelectAndDecodeRawIQ() string {
	if a.ctx == nil {
		return "Error: App context not initialized"
	}

	selectedPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Raw IQ Data File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Raw IQ File (*.iq, *.raw)",
				Pattern:     "*.iq;*.raw",
			},
		},
	})
	if err != nil {
		return fmt.Sprintf("Error selecting file: %v", err)
	}

	if selectedPath == "" {
		return "No file selected"
	}

	return a.DecodeRawIQFile(selectedPath)
}

// DecodeRawIQFile demodulates a raw 8-bit interleaved IQ file and generates decoded APT images
func (a *App) DecodeRawIQFile(iqPath string) string {
	if _, err := os.Stat(iqPath); os.IsNotExist(err) {
		return fmt.Sprintf("Error: File %s does not exist", iqPath)
	}

	cfg := a.store.GetConfig()
	satName := "IQ_Test"

	base := filepath.Base(iqPath)
	ext := filepath.Ext(base)
	nameWithoutExt := base[:len(base)-len(ext)]
	if len(nameWithoutExt) > 0 {
		satName = fmt.Sprintf("IQ_%s", nameWithoutExt)
	}

	baseName := fmt.Sprintf("%s_%s", satName, time.Now().Format("20060102_150405"))
	outDir := cfg.DataDir + "/images"
	audioDir := cfg.DataDir + "/audio"

	logger.Infof("APP", "Starting Raw IQ offline demodulation & decode: file=%s, baseName=%s", iqPath, baseName)

	go func() {
		f, err := os.Open(iqPath)
		if err != nil {
			logger.Errorf("APP", "Failed to open raw IQ file %s: %v", iqPath, err)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "decode_error", err.Error())
			}
			return
		}
		defer f.Close()

		dspEngine := dsp.NewDSP(cfg.SampleRate, 20800)
		buf := make([]byte, 1024*128)
		var pcmSamples []float64

		for {
			n, err := f.Read(buf)
			if n > 0 {
				iqComplex := dsp.ConvertIQToComplex(buf[:n])
				audio := dspEngine.DemodulateFM(iqComplex)
				pcmSamples = append(pcmSamples, audio...)
			}
			if err != nil {
				break
			}
		}

		if len(pcmSamples) == 0 {
			logger.Errorf("APP", "No samples demodulated from IQ file %s", iqPath)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "decode_error", "Empty IQ file")
			}
			return
		}

		var res *decoder.DecodeResult
		upperBase := strings.ToUpper(base)
		if strings.Contains(upperBase, "ISS") || strings.Contains(upperBase, "SONATE") || strings.Contains(upperBase, "SSTV") {
			res, err = decoder.DecodeSSTV(pcmSamples, 20800, outDir, baseName)
		} else if strings.Contains(upperBase, "METEOR") {
			res, err = decoder.DecodeLRPT(iqPath, cfg.SampleRate, outDir, baseName)
		} else {
			res, err = decoder.DecodeAPT(pcmSamples, 20800, outDir, baseName)
		}
		if err != nil {
			logger.Errorf("APP", "Decode from IQ failed: %v", err)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "decode_error", err.Error())
			}
			return
		}

		// Save intermediate demodulated audio WAV
		_ = os.MkdirAll(audioDir, 0755)
		wavPath := fmt.Sprintf("%s/%s_audio.wav", audioDir, baseName)
		int16Audio := dsp.ConvertFloatToINT16(pcmSamples)
		if err := dsp.SaveWAV(wavPath, 20800, int16Audio); err != nil {
			wavPath = ""
		}

		record := &datastore.PassRecord{
			ID:             baseName,
			SatName:        satName,
			Frequency:      137100000,
			Timestamp:      time.Now(),
			ObserverLat:    cfg.ObserverLat,
			ObserverLon:    cfg.ObserverLon,
			MaxElevation:   90.0,
			MaxElevationAz: 0.0,
			AvgRSSI:        0.0,
			PeakRSSI:       0.0,
			Gain:           cfg.TunerGain,
			GainAuto:       cfg.TunerGainAuto,
			ImagePath:      res.FullImagePath,
			ChannelA:       res.ChanAPath,
			ChannelB:       res.ChanBPath,
			AudioPath:      wavPath,
			RawIQPath:      iqPath,
		}

		_ = a.store.AddRecord(record)
		logger.Infof("APP", "Raw IQ decode completed and record saved: ID=%s, Image=%s", record.ID, record.ImagePath)

		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "decode_complete", record)
		}

		if cfg.AutoAnalyze {
			a.RunAIAnalysis(record.ID, "ja")
		}
	}()

	return fmt.Sprintf("Raw IQ offline decode started for %s...", base)
}

// GetRecordSpectrum calculates and returns spectrum timeline data for a pass record
func (a *App) GetRecordSpectrum(recordID string, preferredSource string) (*dsp.SpectrumData, error) {
	records := a.store.GetRecords()
	var target *datastore.PassRecord
	for _, r := range records {
		if r.ID == recordID {
			target = r
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("record %s not found", recordID)
	}

	cfg := a.store.GetConfig()
	sampleRate := cfg.SampleRate
	if sampleRate <= 0 {
		sampleRate = 2080000
	}

	hasRawIQ := target.RawIQPath != ""
	if hasRawIQ {
		if _, err := os.Stat(target.RawIQPath); os.IsNotExist(err) {
			hasRawIQ = false
		}
	}

	hasWAV := target.AudioPath != ""
	if hasWAV {
		if _, err := os.Stat(target.AudioPath); os.IsNotExist(err) {
			hasWAV = false
		}
	}

	if preferredSource == "raw_iq" || (preferredSource != "audio_wav" && hasRawIQ) {
		if !hasRawIQ {
			return nil, fmt.Errorf("raw IQ file not found for record %s", recordID)
		}
		freq := target.Frequency
		if freq <= 0 {
			freq = 137100000
		}
		logger.Infof("SPECTRUM", "Analyzing Raw IQ spectrum for record %s (%s)", recordID, target.RawIQPath)
		return dsp.AnalyzeRawIQSpectrum(target.RawIQPath, freq, sampleRate, 128, 800)
	}

	if hasWAV {
		logger.Infof("SPECTRUM", "Analyzing Audio WAV spectrum for record %s (%s)", recordID, target.AudioPath)
		return dsp.AnalyzeWAVSpectrum(target.AudioPath, 128, 800)
	}

	return nil, fmt.Errorf("no raw IQ or audio WAV file available for record %s", recordID)
}

// AnalyzeSpectrumFile analyzes a specified Raw IQ or WAV audio file and returns spectrum timeline data
func (a *App) AnalyzeSpectrumFile(filePath string) (*dsp.SpectrumData, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	cfg := a.store.GetConfig()
	sampleRate := cfg.SampleRate
	if sampleRate <= 0 {
		sampleRate = 2080000
	}

	ext := filepath.Ext(filePath)
	if ext == ".iq" || ext == ".raw" {
		logger.Infof("SPECTRUM", "Analyzing external Raw IQ file: %s", filePath)
		return dsp.AnalyzeRawIQSpectrum(filePath, 137100000, sampleRate, 128, 800)
	}

	logger.Infof("SPECTRUM", "Analyzing external WAV file: %s", filePath)
	return dsp.AnalyzeWAVSpectrum(filePath, 128, 800)
}

// SelectSpectrumFile opens a file dialog to select a Raw IQ (.iq/.raw) or WAV (.wav) file
func (a *App) SelectSpectrumFile() string {
	if a.ctx == nil {
		return ""
	}

	selectedPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Raw IQ or WAV Audio File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Raw IQ & WAV Files (*.iq, *.raw, *.wav)",
				Pattern:     "*.iq;*.raw;*.wav",
			},
			{
				DisplayName: "Raw IQ Files (*.iq, *.raw)",
				Pattern:     "*.iq;*.raw",
			},
			{
				DisplayName: "WAV Audio Files (*.wav)",
				Pattern:     "*.wav",
			},
		},
	})
	if err != nil {
		logger.Warnf("APP", "Error selecting spectrum file: %v", err)
		return ""
	}

	return selectedPath
}
