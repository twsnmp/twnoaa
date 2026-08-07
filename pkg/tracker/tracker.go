package tracker

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SatelliteInfo struct {
	Name      string  `json:"name"`
	NORADID   string  `json:"norad_id"`
	Frequency int     `json:"frequency"` // Hz
	FreqMHz   float64 `json:"freq_mhz"`
}

var TargetSatellites = []SatelliteInfo{
	{Name: "NOAA 15", NORADID: "25338", Frequency: 137620000, FreqMHz: 137.620},
	{Name: "NOAA 18", NORADID: "28654", Frequency: 137912500, FreqMHz: 137.9125},
	{Name: "NOAA 19", NORADID: "33591", Frequency: 137100000, FreqMHz: 137.100},
}

type SatelliteState struct {
	Name      string    `json:"name"`
	NORADID   string    `json:"norad_id"`
	Frequency int       `json:"frequency"`
	Elevation float64   `json:"elevation"`
	Azimuth   float64   `json:"azimuth"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude"`
	DopplerHz float64   `json:"doppler_hz"`
	IsActive  bool      `json:"is_active"`
	Updated   time.Time `json:"updated"`
}

type PassDetail struct {
	SatName   string    `json:"sat_name"`
	NORADID   string    `json:"norad_id"`
	Frequency int       `json:"frequency"`
	AOS       time.Time `json:"aos"`
	LOS       time.Time `json:"los"`
	MaxElev   float64   `json:"max_elev"`
	MaxElevAz float64   `json:"max_elev_az"`
	Duration  float64   `json:"duration_sec"` // seconds
}

type IPLocation struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	City string  `json:"city"`
}

type Tracker struct {
	mu          sync.RWMutex
	tleMap      map[string]string
	lastTLEFetch time.Time
}

func NewTracker() *Tracker {
	return &Tracker{
		tleMap: make(map[string]string),
	}
}

func GetIPLocation() (*IPLocation, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var loc IPLocation
	if err := json.NewDecoder(resp.Body).Decode(&loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

func (t *Tracker) FetchTLEs() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Cache TLEs for 1 hour
	if time.Since(t.lastTLEFetch) < 1*time.Hour && len(t.tleMap) > 0 {
		return nil
	}

	url := "https://celestrak.org/NORAD/elements/gp.php?GROUP=weather&FORMAT=TLE"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch TLE from Celestrak: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	newMap := make(map[string]string)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "2 ") && i >= 2 {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				noradID := fields[1]
				for _, sat := range TargetSatellites {
					if sat.NORADID == noradID {
						newMap[noradID+"_1"] = strings.TrimSpace(lines[i-1])
						newMap[noradID+"_2"] = line
					}
				}
			}
		}
	}

	t.tleMap = newMap
	t.lastTLEFetch = time.Now()
	return nil
}

func (t *Tracker) GetCurrentStates(obsLat, obsLon, obsAlt float64) ([]*SatelliteState, error) {
	if err := t.FetchTLEs(); err != nil && len(t.tleMap) == 0 {
		return nil, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now().UTC()
	states := make([]*SatelliteState, 0, len(TargetSatellites))

	for _, satInfo := range TargetSatellites {
		tle1, ok1 := t.tleMap[satInfo.NORADID+"_1"]
		tle2, ok2 := t.tleMap[satInfo.NORADID+"_2"]
		if !ok1 || !ok2 {
			continue
		}

		sat, err := parseTLE(tle1, tle2)
		if err != nil {
			continue
		}

		ecef := sat.propagateECEF(now)
		look := calculateLookAngle(obsLat, obsLon, obsAlt, ecef)
		doppler := calculateDopplerShift(satInfo.Frequency, sat, obsLat, obsLon, obsAlt, now)
		lat, lon, alt := ecefToGeodetic(ecef)

		states = append(states, &SatelliteState{
			Name:      satInfo.Name,
			NORADID:   satInfo.NORADID,
			Frequency: satInfo.Frequency,
			Elevation: math.Round(look.Elevation*10) / 10,
			Azimuth:   math.Round(look.Azimuth*10) / 10,
			Latitude:  math.Round(lat*100) / 100,
			Longitude: math.Round(lon*100) / 100,
			Altitude:  math.Round(alt*10) / 10,
			DopplerHz: math.Round(doppler),
			IsActive:  look.Elevation >= 10.0,
			Updated:   time.Now(),
		})
	}

	return states, nil
}

func (t *Tracker) GetUpcomingPasses(obsLat, obsLon, obsAlt float64, hours int) ([]*PassDetail, error) {
	if err := t.FetchTLEs(); err != nil && len(t.tleMap) == 0 {
		return nil, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	startTime := time.Now().UTC()
	endTime := startTime.Add(time.Duration(hours) * time.Hour)

	passes := make([]*PassDetail, 0)

	for _, satInfo := range TargetSatellites {
		tle1, ok1 := t.tleMap[satInfo.NORADID+"_1"]
		tle2, ok2 := t.tleMap[satInfo.NORADID+"_2"]
		if !ok1 || !ok2 {
			continue
		}

		sat, err := parseTLE(tle1, tle2)
		if err != nil {
			continue
		}

		inPass := false
		var passStart time.Time
		var maxEl float64
		var maxElAz float64

		for tm := startTime; tm.Before(endTime); tm = tm.Add(10 * time.Second) {
			ecef := sat.propagateECEF(tm)
			look := calculateLookAngle(obsLat, obsLon, obsAlt, ecef)

			if look.Elevation >= 10.0 {
				if !inPass {
					inPass = true
					passStart = tm
					maxEl = look.Elevation
					maxElAz = look.Azimuth
				}
				if look.Elevation > maxEl {
					maxEl = look.Elevation
					maxElAz = look.Azimuth
				}
			} else {
				if inPass {
					inPass = false
					passes = append(passes, &PassDetail{
						SatName:   satInfo.Name,
						NORADID:   satInfo.NORADID,
						Frequency: satInfo.Frequency,
						AOS:       passStart.Local(),
						LOS:       tm.Local(),
						MaxElev:   math.Round(maxEl*10) / 10,
						MaxElevAz: math.Round(maxElAz*10) / 10,
						Duration:  tm.Sub(passStart).Seconds(),
					})
				}
			}
		}
	}

	return passes, nil
}

// SGP4 Core Engine
type TLESatellite struct {
	EpochYear  int
	EpochDay   float64
	Inclin     float64
	RAAN       float64
	Eccen      float64
	ArgPer     float64
	MeanAnom   float64
	MeanMotion float64
}

func parseTLE(l1, l2 string) (*TLESatellite, error) {
	if len(l1) < 68 || len(l2) < 68 {
		return nil, fmt.Errorf("invalid TLE length")
	}

	year, _ := strconv.Atoi(l1[18:20])
	if year < 50 {
		year += 2000
	} else {
		year += 1900
	}
	day, _ := strconv.ParseFloat(strings.TrimSpace(l1[20:32]), 64)

	inc, _ := strconv.ParseFloat(strings.TrimSpace(l2[8:16]), 64)
	raan, _ := strconv.ParseFloat(strings.TrimSpace(l2[17:25]), 64)
	eccStr := "0." + strings.TrimSpace(l2[26:33])
	ecc, _ := strconv.ParseFloat(eccStr, 64)
	argp, _ := strconv.ParseFloat(strings.TrimSpace(l2[34:42]), 64)
	ma, _ := strconv.ParseFloat(strings.TrimSpace(l2[43:51]), 64)
	mm, _ := strconv.ParseFloat(strings.TrimSpace(l2[52:63]), 64)

	deg2rad := math.Pi / 180.0
	return &TLESatellite{
		EpochYear:  year,
		EpochDay:   day,
		Inclin:     inc * deg2rad,
		RAAN:       raan * deg2rad,
		Eccen:      ecc,
		ArgPer:     argp * deg2rad,
		MeanAnom:   ma * deg2rad,
		MeanMotion: mm * 2.0 * math.Pi / 1440.0,
	}, nil
}

type Vector struct{ X, Y, Z float64 }
type LookAngle struct{ Azimuth, Elevation float64 }

func (s *TLESatellite) propagateECEF(t time.Time) Vector {
	epochTime := time.Date(s.EpochYear, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration((s.EpochDay - 1) * 24 * float64(time.Hour)))
	dtMinutes := t.Sub(epochTime).Minutes()

	M := math.Mod(s.MeanAnom+s.MeanMotion*dtMinutes, 2*math.Pi)
	if M < 0 {
		M += 2 * math.Pi
	}

	E := M
	for i := 0; i < 10; i++ {
		E = E - (E-s.Eccen*math.Sin(E)-M)/(1.0-s.Eccen*math.Cos(E))
	}

	a := math.Pow(398600.4418/(s.MeanMotion*s.MeanMotion/3600.0), 1.0/3.0)
	xOrb := a * (math.Cos(E) - s.Eccen)
	yOrb := a * math.Sqrt(1.0-s.Eccen*s.Eccen) * math.Sin(E)

	u := math.Atan2(yOrb, xOrb) + s.ArgPer
	r := math.Sqrt(xOrb*xOrb + yOrb*yOrb)

	xECI := r * (math.Cos(s.RAAN)*math.Cos(u) - math.Sin(s.RAAN)*math.Sin(u)*math.Cos(s.Inclin))
	yECI := r * (math.Sin(s.RAAN)*math.Cos(u) + math.Cos(s.RAAN)*math.Sin(u)*math.Cos(s.Inclin))
	zECI := r * (math.Sin(u) * math.Sin(s.Inclin))

	julianDay := float64(t.Unix())/86400.0 + 2440587.5
	d := julianDay - 2451545.0
	gst := math.Mod(280.46061837+360.98564736629*d, 360.0) * math.Pi / 180.0

	xECEF := xECI*math.Cos(gst) + yECI*math.Sin(gst)
	yECEF := -xECI*math.Sin(gst) + yECI*math.Cos(gst)
	zECEF := zECI

	return Vector{X: xECEF, Y: yECEF, Z: zECEF}
}

func calculateLookAngle(lat, lon, alt float64, sat Vector) LookAngle {
	radLat := lat * math.Pi / 180.0
	radLon := lon * math.Pi / 180.0

	a := 6378.137
	f := 1.0 / 298.257223563
	e2 := 2*f - f*f

	N := a / math.Sqrt(1-e2*math.Sin(radLat)*math.Sin(radLat))
	obsX := (N + alt) * math.Cos(radLat) * math.Cos(radLon)
	obsY := (N + alt) * math.Cos(radLat) * math.Sin(radLon)
	obsZ := (N*(1-e2) + alt) * math.Sin(radLat)

	rx := sat.X - obsX
	ry := sat.Y - obsY
	rz := sat.Z - obsZ

	topS := math.Sin(radLat)*math.Cos(radLon)*rx + math.Sin(radLat)*math.Sin(radLon)*ry - math.Cos(radLat)*rz
	topE := -math.Sin(radLon)*rx + math.Cos(radLon)*ry
	topU := math.Cos(radLat)*math.Cos(radLon)*rx + math.Cos(radLat)*math.Sin(radLon)*ry + math.Sin(radLat)*rz

	r := math.Sqrt(topE*topE + topS*topS + topU*topU)
	el := math.Asin(topU/r) * 180.0 / math.Pi
	az := math.Atan2(topE, -topS) * 180.0 / math.Pi
	if az < 0 {
		az += 360.0
	}

	return LookAngle{Azimuth: az, Elevation: el}
}

func calculateDopplerShift(freq int, sat *TLESatellite, lat, lon, alt float64, t time.Time) float64 {
	dt := 1.0 // sec
	pos1 := sat.propagateECEF(t)
	pos2 := sat.propagateECEF(t.Add(time.Duration(dt) * time.Second))

	look1 := calculateLookAngle(lat, lon, alt, pos1)
	look2 := calculateLookAngle(lat, lon, alt, pos2)

	// Approximate radial velocity change
	r1 := rangeDistance(lat, lon, alt, pos1)
	r2 := rangeDistance(lat, lon, alt, pos2)
	vRad := (r2 - r1) / dt // km/s

	c := 299792.458 // km/s
	dopplerHz := -float64(freq) * (vRad / c)
	_ = look1
	_ = look2
	return dopplerHz
}

func rangeDistance(lat, lon, alt float64, sat Vector) float64 {
	radLat := lat * math.Pi / 180.0
	radLon := lon * math.Pi / 180.0

	a := 6378.137
	f := 1.0 / 298.257223563
	e2 := 2*f - f*f

	N := a / math.Sqrt(1-e2*math.Sin(radLat)*math.Sin(radLat))
	obsX := (N + alt) * math.Cos(radLat) * math.Cos(radLon)
	obsY := (N + alt) * math.Cos(radLat) * math.Sin(radLon)
	obsZ := (N*(1-e2) + alt) * math.Sin(radLat)

	dx := sat.X - obsX
	dy := sat.Y - obsY
	dz := sat.Z - obsZ
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func ecefToGeodetic(v Vector) (lat, lon, alt float64) {
	a := 6378.137
	f := 1.0 / 298.257223563
	e2 := 2*f - f*f

	lon = math.Atan2(v.Y, v.X) * 180.0 / math.Pi

	p := math.Sqrt(v.X*v.X + v.Y*v.Y)
	lat = math.Atan2(v.Z, p*(1-e2)) * 180.0 / math.Pi

	for i := 0; i < 5; i++ {
		radLat := lat * math.Pi / 180.0
		N := a / math.Sqrt(1-e2*math.Sin(radLat)*math.Sin(radLat))
		alt = p/math.Cos(radLat) - N
		lat = math.Atan2(v.Z, p*(1-e2*(N/(N+alt)))) * 180.0 / math.Pi
	}

	return lat, lon, alt
}
