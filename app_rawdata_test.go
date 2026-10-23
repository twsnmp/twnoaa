package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"twnoaa/pkg/datastore"
)

func setupTestApp(t *testing.T) (*App, string, func()) {
	tempDir, err := os.MkdirTemp("", "twnoaa_app_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := datastore.NewStorePath(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	cfg.DataDir = tempDir
	if err := store.UpdateConfig(cfg); err != nil {
		store.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to update config: %v", err)
	}

	app := &App{
		store: store,
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return app, tempDir, cleanup
}

func TestDeletePassRawIQ(t *testing.T) {
	app, tempDir, cleanup := setupTestApp(t)
	defer cleanup()

	rawDir := filepath.Join(tempDir, "raw_iq")
	if err := os.MkdirAll(rawDir, 0755); err != nil {
		t.Fatalf("Failed to create raw_iq dir: %v", err)
	}

	iqFile := filepath.Join(rawDir, "test_pass_raw.iq")
	if err := os.WriteFile(iqFile, []byte("dummy raw iq data"), 0644); err != nil {
		t.Fatalf("Failed to write dummy IQ file: %v", err)
	}

	rec := &datastore.PassRecord{
		ID:        "rec1",
		SatName:   "NOAA 19",
		Timestamp: time.Now(),
		RawIQPath: iqFile,
		ImagePath: filepath.Join(tempDir, "dummy_image.png"),
	}
	if err := app.store.AddRecord(rec); err != nil {
		t.Fatalf("Failed to add record: %v", err)
	}

	// 1. Delete Raw IQ
	msg := app.DeletePassRawIQ("rec1")
	if msg != "Raw IQ data deleted successfully" {
		t.Errorf("Unexpected result: %s", msg)
	}

	// Check file is removed
	if _, err := os.Stat(iqFile); !os.IsNotExist(err) {
		t.Errorf("Expected raw IQ file to be deleted from disk")
	}

	// Check record in datastore has RawIQPath cleared
	updatedRecords := app.store.GetRecords()
	if len(updatedRecords) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(updatedRecords))
	}
	if updatedRecords[0].RawIQPath != "" {
		t.Errorf("Expected RawIQPath to be empty, got %s", updatedRecords[0].RawIQPath)
	}
	if updatedRecords[0].SatName != "NOAA 19" {
		t.Errorf("Expected SatName NOAA 19 preserved, got %s", updatedRecords[0].SatName)
	}

	// 2. Try deleting again (should report no raw IQ)
	msg2 := app.DeletePassRawIQ("rec1")
	if msg2 != "No raw IQ data associated with this record" {
		t.Errorf("Unexpected result for second delete: %s", msg2)
	}
}

func TestDeletePassRecordRemovesRawIQ(t *testing.T) {
	app, tempDir, cleanup := setupTestApp(t)
	defer cleanup()

	rawDir := filepath.Join(tempDir, "raw_iq")
	imgDir := filepath.Join(tempDir, "images")
	_ = os.MkdirAll(rawDir, 0755)
	_ = os.MkdirAll(imgDir, 0755)

	iqFile := filepath.Join(rawDir, "test_pass_raw.iq")
	imgFile := filepath.Join(imgDir, "test_pass.png")
	_ = os.WriteFile(iqFile, []byte("raw iq data"), 0644)
	_ = os.WriteFile(imgFile, []byte("png data"), 0644)

	rec := &datastore.PassRecord{
		ID:        "rec2",
		SatName:   "METEOR-M2 3",
		RawIQPath: iqFile,
		ImagePath: imgFile,
	}
	_ = app.store.AddRecord(rec)

	// Delete pass record
	msg := app.DeletePassRecord("rec2")
	if msg != "Record deleted successfully" {
		t.Errorf("Unexpected msg: %s", msg)
	}

	// Both files should be deleted
	if _, err := os.Stat(iqFile); !os.IsNotExist(err) {
		t.Errorf("Raw IQ file was not deleted upon DeletePassRecord")
	}
	if _, err := os.Stat(imgFile); !os.IsNotExist(err) {
		t.Errorf("Image file was not deleted upon DeletePassRecord")
	}

	if len(app.store.GetRecords()) != 0 {
		t.Errorf("Expected 0 records in datastore")
	}
}

func TestRawDataStorageInfoAndPurgeAll(t *testing.T) {
	app, tempDir, cleanup := setupTestApp(t)
	defer cleanup()

	rawDir := filepath.Join(tempDir, "raw_iq")
	_ = os.MkdirAll(rawDir, 0755)

	f1 := filepath.Join(rawDir, "file1.iq")
	f2 := filepath.Join(rawDir, "file2.iq")
	_ = os.WriteFile(f1, make([]byte, 1024*1024), 0644)       // 1 MB
	_ = os.WriteFile(f2, make([]byte, 2*1024*1024), 0644)     // 2 MB

	rec1 := &datastore.PassRecord{ID: "p1", SatName: "NOAA 18", RawIQPath: f1}
	rec2 := &datastore.PassRecord{ID: "p2", SatName: "NOAA 19", RawIQPath: f2}
	_ = app.store.AddRecord(rec1)
	_ = app.store.AddRecord(rec2)

	// 1. Get storage info
	info, err := app.GetRawDataStorageInfo()
	if err != nil {
		t.Fatalf("Failed to get raw data storage info: %v", err)
	}
	if info.FileCount != 2 {
		t.Errorf("Expected FileCount 2, got %d", info.FileCount)
	}
	expectedBytes := int64(3 * 1024 * 1024)
	if info.TotalSizeBytes != expectedBytes {
		t.Errorf("Expected %d bytes, got %d", expectedBytes, info.TotalSizeBytes)
	}
	if info.TotalSizeFormatted != "3.00 MB" {
		t.Errorf("Expected '3.00 MB', got '%s'", info.TotalSizeFormatted)
	}

	// 2. Delete All Raw Data
	purgeMsg := app.DeleteAllRawData()
	if purgeMsg != "All raw IQ data deleted (2 files removed)" {
		t.Errorf("Unexpected purge message: %s", purgeMsg)
	}

	// Check files deleted
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Errorf("f1 still exists")
	}
	if _, err := os.Stat(f2); !os.IsNotExist(err) {
		t.Errorf("f2 still exists")
	}

	// Check datastore records updated
	records := app.store.GetRecords()
	for _, r := range records {
		if r.RawIQPath != "" {
			t.Errorf("Record %s still has RawIQPath %s", r.ID, r.RawIQPath)
		}
	}

	// Check storage info is now 0
	infoAfter, _ := app.GetRawDataStorageInfo()
	if infoAfter.FileCount != 0 || infoAfter.TotalSizeBytes != 0 {
		t.Errorf("Expected 0 files and 0 bytes, got %d files, %d bytes", infoAfter.FileCount, infoAfter.TotalSizeBytes)
	}
}

func TestDeletePassesRawIQ(t *testing.T) {
	app, tempDir, cleanup := setupTestApp(t)
	defer cleanup()

	rawDir := filepath.Join(tempDir, "raw_iq")
	_ = os.MkdirAll(rawDir, 0755)

	f1 := filepath.Join(rawDir, "batch1.iq")
	f2 := filepath.Join(rawDir, "batch2.iq")
	_ = os.WriteFile(f1, []byte("data1"), 0644)
	_ = os.WriteFile(f2, []byte("data2"), 0644)

	rec1 := &datastore.PassRecord{ID: "b1", RawIQPath: f1}
	rec2 := &datastore.PassRecord{ID: "b2", RawIQPath: f2}
	_ = app.store.AddRecord(rec1)
	_ = app.store.AddRecord(rec2)

	res := app.DeletePassesRawIQ([]string{"b1", "b2"})
	if res != "Deleted raw IQ data for 2 records" {
		t.Errorf("Unexpected batch delete result: %s", res)
	}

	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Errorf("f1 still exists")
	}
	if _, err := os.Stat(f2); !os.IsNotExist(err) {
		t.Errorf("f2 still exists")
	}
}
