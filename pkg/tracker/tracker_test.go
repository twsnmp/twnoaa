package tracker

import (
	"testing"
	"time"
)

func TestParseTLE(t *testing.T) {
	line1 := "1 25338U 98030A   24001.50000000  .00000100  00000-0  50000-4 0  9999"
	line2 := "2 25338  98.7000 100.0000 0010000  90.0000 270.0000 14.12000000100003"

	sat, err := parseTLE(line1, line2)
	if err != nil {
		t.Fatalf("Failed to parse TLE: %v", err)
	}

	if sat.EpochYear != 2024 {
		t.Errorf("Expected year 2024, got %d", sat.EpochYear)
	}

	ecef := sat.propagateECEF(time.Now())
	if ecef.X == 0 && ecef.Y == 0 && ecef.Z == 0 {
		t.Errorf("Propagated position is zero vector")
	}

	obs := NewObserver(36.028, 139.533, 0.02)
	look := obs.LookAngle(ecef)
	if look.Azimuth < 0 || look.Azimuth > 360 {
		t.Errorf("Azimuth out of bounds: %f", look.Azimuth)
	}
}

func TestTargetSatellitesList(t *testing.T) {
	if len(TargetSatellites) != 3 {
		t.Fatalf("Expected 3 target satellites, got %d", len(TargetSatellites))
	}
	if TargetSatellites[0].Name != "NOAA 15" {
		t.Errorf("Expected NOAA 15 first, got %s", TargetSatellites[0].Name)
	}
}

func TestFetchAndCalculateUpcomingPasses(t *testing.T) {
	start := time.Now()
	tr := NewTracker()

	passes, err := tr.GetUpcomingPasses(36.028, 139.533, 0.02, 24)
	if err != nil {
		t.Fatalf("GetUpcomingPasses returned error: %v", err)
	}

	elapsed := time.Since(start)
	t.Logf("Calculated %d passes in %v", len(passes), elapsed)
	for i, p := range passes {
		t.Logf("Pass #%d: %s | AOS: %s | LOS: %s | MaxElev: %.1f° | Azimuth: %.1f°",
			i+1, p.SatName, p.AOS.Format("15:04:05"), p.LOS.Format("15:04:05"), p.MaxElev, p.MaxElevAz)
	}
}
