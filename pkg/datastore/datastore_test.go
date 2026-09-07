package datastore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestDatastoreBbolt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "twnoaa_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStorePath(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Verify default config
	cfg := store.GetConfig()
	if cfg.ActiveProvider != "ollama" {
		t.Errorf("Expected ActiveProvider 'ollama', got '%s'", cfg.ActiveProvider)
	}
	if cfg.DataDir != tempDir {
		t.Errorf("Expected DataDir '%s', got '%s'", tempDir, cfg.DataDir)
	}

	// 2. Update config with sensitive API key and verify encryption at rest
	cfg.APIKeyGemini = "secret_gemini_key_123"
	cfg.ObserverCity = "Tokyo"
	if err := store.UpdateConfig(cfg); err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Re-get config (should be decrypted transparently)
	updatedCfg := store.GetConfig()
	if updatedCfg.APIKeyGemini != "secret_gemini_key_123" {
		t.Errorf("Expected APIKeyGemini 'secret_gemini_key_123', got '%s'", updatedCfg.APIKeyGemini)
	}
	if updatedCfg.ObserverCity != "Tokyo" {
		t.Errorf("Expected ObserverCity 'Tokyo', got '%s'", updatedCfg.ObserverCity)
	}

	// Check raw bbolt data to ensure API key is encrypted on disk
	err = store.conn.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketConfig)
		data := b.Get(configKey)
		if len(data) > 0 {
			if strings.Contains(string(data), "secret_gemini_key_123") {
				t.Errorf("Plaintext API key was found in raw bbolt database!")
			}
			if !strings.Contains(string(data), encPrefix) {
				t.Errorf("Expected enc: prefix in stored ciphertext")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Error viewing bbolt DB: %v", err)
	}

	// 3. Add & Retrieve PassRecords
	rec1 := &PassRecord{
		ID:           "NOAA18_20260808_120000",
		SatName:      "NOAA 18",
		Frequency:    137912500,
		Timestamp:    time.Now().Add(-1 * time.Hour),
		ObserverLat:  35.68,
		ObserverLon:  139.76,
		MaxElevation: 50.0,
		ImagePath:    filepath.Join(tempDir, "images", "rec1.png"),
	}
	rec2 := &PassRecord{
		ID:           "NOAA19_20260808_130000",
		SatName:      "NOAA 19",
		Frequency:    137100000,
		Timestamp:    time.Now(),
		ObserverLat:  35.68,
		ObserverLon:  139.76,
		MaxElevation: 75.0,
		ImagePath:    filepath.Join(tempDir, "images", "rec2.png"),
	}

	if err := store.AddRecord(rec1); err != nil {
		t.Fatalf("Failed to add rec1: %v", err)
	}
	if err := store.AddRecord(rec2); err != nil {
		t.Fatalf("Failed to add rec2: %v", err)
	}

	records := store.GetRecords()
	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}
	// Verify sorted newest first
	if records[0].ID != rec2.ID {
		t.Errorf("Expected first record to be rec2 (%s), got %s", rec2.ID, records[0].ID)
	}

	// 4. Update record (e.g. AI analysis)
	rec2.AIAnalysis = "Cloudy over Pacific"
	if err := store.UpdateRecord(rec2); err != nil {
		t.Fatalf("Failed to update rec2: %v", err)
	}

	recordsAfterUpdate := store.GetRecords()
	if recordsAfterUpdate[0].AIAnalysis != "Cloudy over Pacific" {
		t.Errorf("Expected AIAnalysis 'Cloudy over Pacific', got '%s'", recordsAfterUpdate[0].AIAnalysis)
	}

	// 5. Delete record
	if err := store.DeleteRecord(rec1.ID); err != nil {
		t.Fatalf("Failed to delete rec1: %v", err)
	}

	recordsAfterDelete := store.GetRecords()
	if len(recordsAfterDelete) != 1 {
		t.Fatalf("Expected 1 record after delete, got %d", len(recordsAfterDelete))
	}
	if recordsAfterDelete[0].ID != rec2.ID {
		t.Errorf("Expected remaining record to be rec2 (%s), got %s", rec2.ID, recordsAfterDelete[0].ID)
	}

	// 6. Delete multiple records
	rec3 := &PassRecord{
		ID:        "test-pass-003",
		SatName:   "NOAA 18",
		Timestamp: time.Now().Add(2 * time.Minute),
	}
	_ = store.AddRecord(rec3)
	if err := store.DeleteRecords([]string{rec2.ID, rec3.ID}); err != nil {
		t.Fatalf("Failed to delete records: %v", err)
	}
	if len(store.GetRecords()) != 0 {
		t.Errorf("Expected 0 records after batch delete, got %d", len(store.GetRecords()))
	}
}

