package datastore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	// Observer location
	ObserverLat  float64 `json:"observer_lat"`
	ObserverLon  float64 `json:"observer_lon"`
	ObserverAlt  float64 `json:"observer_alt"`
	ObserverCity string  `json:"observer_city"`

	// LLM settings
	ActiveProvider  string `json:"active_provider"`   // gemini | openai | ollama | anthropic
	APIKeyGemini    string `json:"api_key_gemini"`
	APIKeyOpenAI    string `json:"api_key_openai"`
	APIKeyAnthropic string `json:"api_key_anthropic"`
	OllamaURL       string `json:"ollama_url"`
	OllamaModel     string `json:"ollama_model"`
	AutoAnalyze     bool   `json:"auto_analyze"`

	// SDR Receiver settings
	CenterFreq     int  `json:"center_freq"`
	SampleRate     int  `json:"sample_rate"`
	TunerGain      int  `json:"tuner_gain"`
	TunerGainAuto  bool `json:"tuner_gain_auto"`
	SimulationMode bool `json:"simulation_mode"`

	// Data storage
	DataDir string `json:"data_dir"`
}

type PassRecord struct {
	ID           string    `json:"id"`
	SatName      string    `json:"sat_name"`
	Frequency    int       `json:"frequency"`
	Timestamp    time.Time `json:"timestamp"`
	ObserverLat  float64   `json:"observer_lat"`
	ObserverLon  float64   `json:"observer_lon"`
	MaxElevation float64   `json:"max_elevation"`
	ImagePath    string    `json:"image_path"`
	ChannelA     string    `json:"channel_a"`
	ChannelB     string    `json:"channel_b"`
	AIAnalysis   string    `json:"ai_analysis"`
	AnalysisDate time.Time `json:"analysis_date"`
}

type Store struct {
	mu         sync.RWMutex
	config     Config
	records    []*PassRecord
	configPath string
}

func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	baseDir := filepath.Join(homeDir, ".twnoaa")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		baseDir = "."
	}

	imagesDir := filepath.Join(baseDir, "images")
	_ = os.MkdirAll(imagesDir, 0755)

	configPath := filepath.Join(baseDir, "config.json")

	store := &Store{
		configPath: configPath,
		config: Config{
			ObserverLat:    36.028, // Default: Kitamoto, Saitama
			ObserverLon:    139.533,
			ObserverAlt:    0.02,
			ObserverCity:   "Kitamoto, Saitama",
			ActiveProvider: "gemini",
			OllamaURL:      "http://localhost:11434",
			OllamaModel:    "llama3",
			AutoAnalyze:    true,
			CenterFreq:     137100000,
			SampleRate:     2080000,
			TunerGain:      400,
			TunerGainAuto:  false,
			SimulationMode: true, // Default to simulation if no hardware connected
			DataDir:        baseDir,
		},
		records: make([]*PassRecord, 0),
	}

	_ = store.Load()
	return store, nil
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	var wrapper struct {
		Config  Config        `json:"config"`
		Records []*PassRecord `json:"records"`
	}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}

	s.config = wrapper.Config
	s.records = wrapper.Records
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wrapper := struct {
		Config  Config        `json:"config"`
		Records []*PassRecord `json:"records"`
	}{
		Config:  s.config,
		Records: s.records,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.configPath, data, 0644)
}

func (s *Store) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *Store) UpdateConfig(cfg Config) error {
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) GetRecords() []*PassRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]*PassRecord, len(s.records))
	copy(res, s.records)
	return res
}

func (s *Store) AddRecord(rec *PassRecord) error {
	s.mu.Lock()
	s.records = append([]*PassRecord{rec}, s.records...)
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) UpdateRecord(rec *PassRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.records {
		if r.ID == rec.ID {
			s.records[i] = rec
			break
		}
	}
	return s.Save()
}
