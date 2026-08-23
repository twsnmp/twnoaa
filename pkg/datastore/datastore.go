package datastore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketConfig  = []byte("config")
	bucketRecords = []byte("records")

	configKey = []byte("system")
)

type Config struct {
	// Observer location
	ObserverLat  float64 `json:"observer_lat"`
	ObserverLon  float64 `json:"observer_lon"`
	ObserverAlt  float64 `json:"observer_alt"`
	ObserverCity string  `json:"observer_city"`

	// LLM settings
	ActiveProvider  string `json:"active_provider"` // gemini | openai | ollama | anthropic
	APIKeyGemini    string `json:"api_key_gemini"`
	APIKeyOpenAI    string `json:"api_key_openai"`
	APIKeyAnthropic string `json:"api_key_anthropic"`
	OllamaURL       string `json:"ollama_url"`
	OllamaModel     string `json:"ollama_model"`
	AutoAnalyze     bool   `json:"auto_analyze"`

	// SDR Receiver settings
	CenterFreq         int  `json:"center_freq"`
	SampleRate         int  `json:"sample_rate"`
	TunerGain          int  `json:"tuner_gain"`
	TunerGainAuto      bool `json:"tuner_gain_auto"`
	BiasTee            bool `json:"bias_tee"`
	SimulationMode     bool `json:"simulation_mode"`
	AutoRecord         bool `json:"auto_record"`
	AutoRecordDuration int  `json:"auto_record_duration"`

	// Data storage
	DataDir string `json:"data_dir"`
}

type SignalPoint struct {
	TimeSec float64 `json:"time_sec"`
	Signal  float64 `json:"signal"`
	Noise   float64 `json:"noise"`
	SNR     float64 `json:"snr"`
}

type PassRecord struct {
	ID            string        `json:"id"`
	SatName       string        `json:"sat_name"`
	Frequency     int           `json:"frequency"`
	Timestamp     time.Time     `json:"timestamp"`
	ObserverLat   float64       `json:"observer_lat"`
	ObserverLon   float64       `json:"observer_lon"`
	MaxElevation  float64       `json:"max_elevation"`
	MaxElevationAz float64      `json:"max_elevation_az"`
	AvgRSSI       float64       `json:"avg_rssi"`
	PeakRSSI      float64       `json:"peak_rssi"`
	ImagePath     string        `json:"image_path"`
	ChannelA      string        `json:"channel_a"`
	ChannelB      string        `json:"channel_b"`
	AudioPath     string        `json:"audio_path"`
	AIAnalysis    string        `json:"ai_analysis"`
	AnalysisDate  time.Time     `json:"analysis_date"`
	SignalHistory []SignalPoint `json:"signal_history,omitempty"`
}

type Store struct {
	conn *bolt.DB
	mu   sync.Mutex
	key  []byte
}

func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	baseDir := filepath.Join(homeDir, ".twnoaa")
	return NewStorePath(baseDir)
}

func NewStorePath(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		baseDir = "."
	}

	imagesDir := filepath.Join(baseDir, "images")
	_ = os.MkdirAll(imagesDir, 0755)

	dbPath := filepath.Join(baseDir, "twnoaa.db")
	conn, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt db at %s: %w", dbPath, err)
	}

	key, err := loadOrCreateKey(baseDir)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize encryption key: %w", err)
	}

	store := &Store{
		conn: conn,
		key:  key,
	}

	if err := store.initBuckets(baseDir); err != nil {
		conn.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) initBuckets(baseDir string) error {
	return s.conn.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{bucketConfig, bucketRecords}
		for _, b := range buckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", b, err)
			}
		}

		// Initialize default config if not present
		cb := tx.Bucket(bucketConfig)
		if cb.Get(configKey) == nil {
			defaultCfg := Config{
				ObserverLat:        36.028, // Default: Kitamoto, Saitama
				ObserverLon:        139.533,
				ObserverAlt:        0.02,
				ObserverCity:       "Kitamoto, Saitama",
				ActiveProvider:     "ollama",
				OllamaURL:          "http://localhost:11434",
				OllamaModel:        "llama3",
				AutoAnalyze:        true,
				CenterFreq:         137100000,
				SampleRate:         2080000,
				TunerGain:          400,
				TunerGainAuto:      false,
				BiasTee:            false,
				SimulationMode:     false, // Default to hardware mode (falls back automatically if no hardware)
				AutoRecord:         false, // Default OFF
				AutoRecordDuration: 30,
				DataDir:            baseDir,
			}
			data, err := json.Marshal(&defaultCfg)
			if err != nil {
				return err
			}
			if err := cb.Put(configKey, data); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *Store) GetConfig() Config {
	defaultCfg := Config{
		ObserverLat:        36.028,
		ObserverLon:        139.533,
		ObserverAlt:        0.02,
		ObserverCity:       "Kitamoto, Saitama",
		ActiveProvider:     "ollama",
		OllamaURL:          "http://localhost:11434",
		OllamaModel:        "llama3",
		AutoAnalyze:        true,
		CenterFreq:         137100000,
		SampleRate:         2080000,
		TunerGain:          380, // 38.0 dB optimal gain for dipole antenna without external LNA
		TunerGainAuto:      false,
		BiasTee:            false,
		SimulationMode:     false,
		AutoRecord:         false,
		AutoRecordDuration: 30,
	}

	if s == nil || s.conn == nil {
		return defaultCfg
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var cfg Config

	err := s.conn.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		if b == nil {
			cfg = defaultCfg
			return nil
		}
		data := b.Get(configKey)
		if data == nil {
			cfg = defaultCfg
			return nil
		}
		return json.Unmarshal(data, &cfg)
	})

	if err != nil {
		cfg = defaultCfg
	}

	cfg.APIKeyGemini = decryptField(s.key, cfg.APIKeyGemini)
	cfg.APIKeyOpenAI = decryptField(s.key, cfg.APIKeyOpenAI)
	cfg.APIKeyAnthropic = decryptField(s.key, cfg.APIKeyAnthropic)

	return cfg
}

func (s *Store) UpdateConfig(cfg Config) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("datastore is not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	encrypted := cfg
	var err error
	if encrypted.APIKeyGemini, err = encryptField(s.key, cfg.APIKeyGemini); err != nil {
		return fmt.Errorf("failed to encrypt Gemini API key: %w", err)
	}
	if encrypted.APIKeyOpenAI, err = encryptField(s.key, cfg.APIKeyOpenAI); err != nil {
		return fmt.Errorf("failed to encrypt OpenAI API key: %w", err)
	}
	if encrypted.APIKeyAnthropic, err = encryptField(s.key, cfg.APIKeyAnthropic); err != nil {
		return fmt.Errorf("failed to encrypt Anthropic API key: %w", err)
	}

	return s.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		data, err := json.Marshal(&encrypted)
		if err != nil {
			return err
		}
		return b.Put(configKey, data)
	})
}

func (s *Store) GetRecords() []*PassRecord {
	if s == nil || s.conn == nil {
		return []*PassRecord{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records := []*PassRecord{}
	_ = s.conn.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRecords)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var rec PassRecord
			if err := json.Unmarshal(v, &rec); err == nil {
				records = append(records, &rec)
			}
			return nil
		})
	})

	// Sort records newest first by Timestamp
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.After(records[j].Timestamp)
	})

	return records
}

func (s *Store) AddRecord(rec *PassRecord) error {
	return s.UpdateRecord(rec)
}

func (s *Store) UpdateRecord(rec *PassRecord) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("datastore is not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRecords)
		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		return b.Put([]byte(rec.ID), data)
	})
}

func (s *Store) DeleteRecord(id string) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("datastore is not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.conn.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRecords)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
	})
}
