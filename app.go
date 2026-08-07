package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"twnoaa/pkg/ai"
	"twnoaa/pkg/datastore"
	"twnoaa/pkg/decoder"
	"twnoaa/pkg/dsp"
	"twnoaa/pkg/sdr"
	"twnoaa/pkg/tracker"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	store      *datastore.Store
	tracker    *tracker.Tracker
	sdrService *sdr.SDR
	recording  bool
	recMu      sync.Mutex
}

func NewApp() *App {
	st, _ := datastore.NewStore()
	tr := tracker.NewTracker()
	sd := sdr.NewSDR()

	return &App{
		store:      st,
		tracker:    tr,
		sdrService: sd,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Pre-fetch TLEs
	go func() {
		_ = a.tracker.FetchTLEs()
	}()
}

func (a *App) shutdown(ctx context.Context) {
	a.sdrService.StopStream()
}

// Config API
func (a *App) GetConfig() datastore.Config {
	return a.store.GetConfig()
}

func (a *App) SaveConfig(cfg datastore.Config) string {
	if err := a.store.UpdateConfig(cfg); err != nil {
		return fmt.Sprintf("Error saving config: %v", err)
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
		hours = 12
	}
	cfg := a.store.GetConfig()
	return a.tracker.GetUpcomingPasses(cfg.ObserverLat, cfg.ObserverLon, cfg.ObserverAlt, hours)
}

// SDR Receiver API
func (a *App) GetSDRDeviceInfo() sdr.DeviceInfo {
	return sdr.GetDeviceInfo()
}

func (a *App) StartSDR(freq, sampleRate, gain int, autoGain, forceSim bool) string {
	err := a.sdrService.StartStream(freq, sampleRate, gain, autoGain, forceSim)
	if err != nil {
		return fmt.Sprintf("Failed to start SDR: %v", err)
	}

	// Stream waterfall data to frontend
	go func() {
		sub := a.sdrService.Subscribe()
		defer a.sdrService.Unsubscribe(sub)

		for buf := range sub {
			if a.ctx != nil {
				spectrum := dsp.CalculatePowerSpectrum(buf, 128)
				runtime.EventsEmit(a.ctx, "sdr_spectrum", spectrum)
			}
		}
	}()

	return "SDR stream started"
}

func (a *App) StopSDR() string {
	a.sdrService.StopStream()
	return "SDR stream stopped"
}

// Recording & Decoder API
func (a *App) StartRecordingAndDecode(satName string, freq int, durationSec int) string {
	a.recMu.Lock()
	if a.recording {
		a.recMu.Unlock()
		return "Recording is already in progress"
	}
	a.recording = true
	a.recMu.Unlock()

	go func() {
		defer func() {
			a.recMu.Lock()
			a.recording = false
			a.recMu.Unlock()
		}()

		cfg := a.store.GetConfig()
		if !a.sdrService.IsStreaming() {
			_ = a.sdrService.StartStream(freq, cfg.SampleRate, cfg.TunerGain, cfg.TunerGainAuto, cfg.SimulationMode)
		}

		sub := a.sdrService.Subscribe()
		defer a.sdrService.Unsubscribe(sub)

		dspEngine := dsp.NewDSP(cfg.SampleRate, 20800)
		var pcmSamples []float64

		timeout := time.After(time.Duration(durationSec) * time.Second)

		for {
			select {
			case <-timeout:
				goto DONE
			case buf, ok := <-sub:
				if !ok {
					goto DONE
				}
				iqComplex := dsp.ConvertIQToComplex(buf)
				audio := dspEngine.DemodulateFM(iqComplex)
				pcmSamples = append(pcmSamples, audio...)
			}
		}

	DONE:
		baseName := fmt.Sprintf("%s_%s", satName, time.Now().Format("20060102_150405"))
		res, err := decoder.DecodeAPT(pcmSamples, 20800, cfg.DataDir+"/images", baseName)
		if err != nil {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "decode_error", err.Error())
			}
			return
		}

		// Create record
		record := &datastore.PassRecord{
			ID:           baseName,
			SatName:      satName,
			Frequency:    freq,
			Timestamp:    time.Now(),
			ObserverLat:  cfg.ObserverLat,
			ObserverLon:  cfg.ObserverLon,
			MaxElevation: 45.0,
			ImagePath:    res.FullImagePath,
			ChannelA:     res.ChanAPath,
			ChannelB:     res.ChanBPath,
		}

		_ = a.store.AddRecord(record)

		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "decode_complete", record)
		}

		// Auto AI analysis if enabled
		if cfg.AutoAnalyze {
			a.RunAIAnalysis(record.ID)
		}
	}()

	return fmt.Sprintf("Started recording for %d seconds...", durationSec)
}

func (a *App) GetPassRecords() []*datastore.PassRecord {
	return a.store.GetRecords()
}

func (a *App) RunAIAnalysis(recordID string) string {
	records := a.store.GetRecords()
	var target *datastore.PassRecord
	for _, r := range records {
		if r.ID == recordID {
			target = r
			break
		}
	}

	if target == nil {
		return "Record not found"
	}

	cfg := a.store.GetConfig()
	req := ai.AnalysisRequest{
		SatelliteName: target.SatName,
		FrequencyMHz:  float64(target.Frequency) / 1e6,
		TimestampStr:  target.Timestamp.Format("2006-01-02 15:04:05 (JST)"),
		ObserverCity:  cfg.ObserverCity,
		ObserverLat:   target.ObserverLat,
		ObserverLon:   target.ObserverLon,
		MaxElevation:  target.MaxElevation,
		ImagePath:     filepath.Clean(target.ImagePath),
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
