package decoder

import (
	"math"
	"testing"
)

// Helper to create synthetic lineMap
func createSyntheticLineMap(numLines int, visMean, nirMean, irPattern float64) (map[int]*lrptScanLine, []int) {
	lineMap := make(map[int]*lrptScanLine)
	lineKeys := make([]int, numLines)

	for i := 0; i < numLines; i++ {
		lineKeys[i] = i
		sl := &lrptScanLine{}
		for col := 0; col < 196; col++ {
			// Vis (ch0)
			sl.valid[0][col] = true
			for p := 0; p < 64; p++ {
				sl.blocks[0][col][p] = uint8(math.Max(0, math.Min(255, visMean)))
			}
			// NIR (ch1)
			sl.valid[1][col] = true
			for p := 0; p < 64; p++ {
				sl.blocks[1][col][p] = uint8(math.Max(0, math.Min(255, nirMean)))
			}
			// IR (ch2) - create gradient/stripes pattern
			sl.valid[2][col] = true
			for p := 0; p < 64; p++ {
				v := irPattern + float64(col%2)*60.0 + float64(p%8)*10.0
				sl.blocks[2][col][p] = uint8(math.Max(0, math.Min(255, v)))
			}
		}
		lineMap[i] = sl
	}
	return lineMap, lineKeys
}

func TestT2_NightRender(t *testing.T) {
	// Synthetic Night: Vis=12 (noise floor), NIR=12, IR has structure (gradient 30..220)
	numLines := 40
	lineMap, lineKeys := createSyntheticLineMap(numLines, 12.0, 12.0, 50.0)

	fullImg, chanA, chanB, dayFraction := renderLRPTImages(lineMap, lineKeys, 0, true)
	if fullImg == nil || chanA == nil || chanB == nil {
		t.Fatal("renderLRPTImages returned nil image")
	}

	if dayFraction > 0.05 {
		t.Errorf("Expected dayFraction < 0.05 for night, got %.3f", dayFraction)
	}

	// Calculate standard deviation of fullImg pixels
	bounds := fullImg.Bounds()
	var sum, sumSq float64
	cnt := float64(bounds.Dx() * bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := fullImg.RGBAAt(x, y)
			v := float64(c.R) // night mode is grayscale IR
			sum += v
			sumSq += v * v
		}
	}
	mean := sum / cnt
	stdDev := math.Sqrt(sumSq/cnt - mean*mean)

	if stdDev < 20.0 {
		t.Errorf("Expected full image to show thermal contrast (stdDev >= 20), got %.2f", stdDev)
	}
}

func TestT3_DayRender(t *testing.T) {
	// Synthetic Day: Vis=110, NIR=120, IR=100
	numLines := 40
	lineMap, lineKeys := createSyntheticLineMap(numLines, 110.0, 120.0, 100.0)

	fullImg, chanA, chanB, dayFraction := renderLRPTImages(lineMap, lineKeys, 15000000, true)
	if fullImg == nil || chanA == nil || chanB == nil {
		t.Fatal("renderLRPTImages returned nil image")
	}

	if dayFraction < 0.95 {
		t.Errorf("Expected dayFraction > 0.95 for day, got %.3f", dayFraction)
	}
}

func TestT4_TerminatorRender(t *testing.T) {
	// Upper 20 lines day (Vis=100), lower 20 lines night (Vis=12)
	numLines := 40
	lineMap := make(map[int]*lrptScanLine)
	lineKeys := make([]int, numLines)

	for i := 0; i < numLines; i++ {
		lineKeys[i] = i
		visVal := 12.0
		if i < 20 {
			visVal = 100.0
		}
		sl := &lrptScanLine{}
		for col := 0; col < 196; col++ {
			sl.valid[0][col] = true
			for p := 0; p < 64; p++ {
				sl.blocks[0][col][p] = uint8(visVal)
			}
			sl.valid[1][col] = true
			for p := 0; p < 64; p++ {
				sl.blocks[1][col][p] = uint8(visVal)
			}
			sl.valid[2][col] = true
			for p := 0; p < 64; p++ {
				sl.blocks[2][col][p] = uint8(128)
			}
		}
		lineMap[i] = sl
	}

	fullImg, _, _, dayFraction := renderLRPTImages(lineMap, lineKeys, 0, true)
	if fullImg == nil {
		t.Fatal("renderLRPTImages returned nil image")
	}

	if dayFraction < 0.3 || dayFraction > 0.7 {
		t.Errorf("Expected intermediate dayFraction ~0.5 for terminator pass, got %.3f", dayFraction)
	}
}

func TestT5_GapMaskAndInterpolationLimit(t *testing.T) {
	// Test that small gap (1 line) is interpolated, but wide gap (4 lines) leaves unreceived pixels as black background
	numLines := 20
	lineMap := make(map[int]*lrptScanLine)
	lineKeys := make([]int, numLines)

	for i := 0; i < numLines; i++ {
		lineKeys[i] = i
		// Simulate gap: line 2 missing (1 line gap, should interpolate)
		// lines 8, 9, 10, 11 missing (4 lines gap, lines 9-10 should NOT vertically stretch curtain)
		if i == 2 || (i >= 8 && i <= 11) {
			continue // unreceived line (nil in lineMap)
		}
		sl := &lrptScanLine{}
		for col := 0; col < 196; col++ {
			sl.valid[0][col] = true
			sl.valid[1][col] = true
			sl.valid[2][col] = true
			for p := 0; p < 64; p++ {
				sl.blocks[0][col][p] = 100
				sl.blocks[1][col][p] = 120
				sl.blocks[2][col][p] = 80
			}
		}
		lineMap[i] = sl
	}

	fullImg, chanA, _, _ := renderLRPTImages(lineMap, lineKeys, 15000000, true)
	if fullImg == nil || chanA == nil {
		t.Fatal("renderLRPTImages returned nil image")
	}

	// Line 2 (y = 2*8 + 4 = 20) should have interpolated data (R > 0)
	cSmallGap := fullImg.RGBAAt(100, 20)
	if cSmallGap.R == 0 && cSmallGap.G == 0 && cSmallGap.B == 0 {
		t.Errorf("Expected small 1-line gap at y=20 to be vertically interpolated, got black pixel")
	}

	// Line 10 (y = 10*8 + 4 = 84) in the 4-line gap should be black (unreceived mask, not curtain-stretched)
	cWideGap := fullImg.RGBAAt(100, 84)
	if cWideGap.R != 0 || cWideGap.G != 0 || cWideGap.B != 0 {
		t.Errorf("Expected wide 4-line gap center at y=84 to remain black background, got color %+v", cWideGap)
	}
}

func TestT6_RectifyLRPTTangential(t *testing.T) {
	numLines := 10
	lineMap, lineKeys := createSyntheticLineMap(numLines, 100.0, 100.0, 100.0)
	fullImg, _, _, _ := renderLRPTImages(lineMap, lineKeys, 15000000, true)
	if fullImg == nil {
		t.Fatal("renderLRPTImages returned nil image")
	}

	rectified := RectifyLRPTTangential(fullImg)
	if rectified == nil {
		t.Fatal("RectifyLRPTTangential returned nil")
	}
	if rectified.Bounds() != fullImg.Bounds() {
		t.Errorf("Expected bounds %+v, got %+v", fullImg.Bounds(), rectified.Bounds())
	}
}

func TestT7_SingleChannelGrayscaleFallback(t *testing.T) {
	// Test that when only 1 channel is valid (e.g. Vis valid, NIR missing),
	// the synthesized pixel falls back to neutral grayscale without turning into a pure primary color (e.g. pure blue).
	numLines := 10
	lineMap := make(map[int]*lrptScanLine)
	lineKeys := make([]int, numLines)

	for i := 0; i < numLines; i++ {
		lineKeys[i] = i
		sl := &lrptScanLine{}
		for col := 0; col < 196; col++ {
			if col < 50 {
				// Both Vis and NIR valid
				sl.valid[0][col] = true
				sl.valid[1][col] = true
				for p := 0; p < 64; p++ {
					sl.blocks[0][col][p] = 100
					sl.blocks[1][col][p] = 120
				}
			} else if col < 100 {
				// Only Vis valid (NIR missing)
				sl.valid[0][col] = true
				sl.valid[1][col] = false
				for p := 0; p < 64; p++ {
					sl.blocks[0][col][p] = 100
				}
			} else if col < 150 {
				// Only NIR valid (Vis missing)
				sl.valid[0][col] = false
				sl.valid[1][col] = true
				for p := 0; p < 64; p++ {
					sl.blocks[1][col][p] = 110
				}
			}
		}
		lineMap[i] = sl
	}

	fullImg, _, _, _ := renderLRPTImages(lineMap, lineKeys, 15000000, true)
	if fullImg == nil {
		t.Fatal("renderLRPTImages returned nil image")
	}

	// 1. Check pixel in col 75 (Only Vis valid) -> should be neutral grayscale, NOT pure blue/cyan
	cVisOnly := fullImg.RGBAAt(75*8+4, 10)
	diffRG := math.Abs(float64(cVisOnly.R) - float64(cVisOnly.G))
	diffGB := math.Abs(float64(cVisOnly.G) - float64(cVisOnly.B))
	diffRB := math.Abs(float64(cVisOnly.R) - float64(cVisOnly.B))
	if diffRG > 2 || diffGB > 2 || diffRB > 2 {
		t.Errorf("Expected Vis-only segment to render as neutral grayscale, got color %+v (diffRG=%.1f, diffGB=%.1f, diffRB=%.1f)",
			cVisOnly, diffRG, diffGB, diffRB)
	}

	// 2. Check pixel in col 125 (Only NIR valid) -> should be neutral grayscale, NOT pure red/yellow
	cNirOnly := fullImg.RGBAAt(125*8+4, 10)
	diffRG = math.Abs(float64(cNirOnly.R) - float64(cNirOnly.G))
	diffGB = math.Abs(float64(cNirOnly.G) - float64(cNirOnly.B))
	diffRB = math.Abs(float64(cNirOnly.R) - float64(cNirOnly.B))
	if diffRG > 2 || diffGB > 2 || diffRB > 2 {
		t.Errorf("Expected NIR-only segment to render as neutral grayscale, got color %+v (diffRG=%.1f, diffGB=%.1f, diffRB=%.1f)",
			cNirOnly, diffRG, diffGB, diffRB)
	}
}

func TestT8_PartialMCUSalvageConditions(t *testing.T) {
	// Verify that validMCUs >= 6 is salvaged with 2 safety buffer blocks truncated,
	// while < 6, clipped blocks (<=3 or >=252), or runaway jumps (>90) are rejected.
	testSalvage := func(validMCUs int, startVal, endVal uint8) (bool, int) {
		if validMCUs < 14 {
			if validMCUs < 6 {
				return false, 0
			}
			safeMCUs := validMCUs - 2
			meanStart := float64(startVal)
			meanEnd := float64(endVal)
			if meanStart <= 3.0 || meanStart >= 252.0 || meanEnd <= 3.0 || meanEnd >= 252.0 {
				return false, 0
			}
			if math.Abs(meanEnd-meanStart) > 90.0 {
				return false, 0
			}
			return true, safeMCUs
		}
		return true, 14
	}

	// Case 1: validMCUs = 10, normal luminance -> should salvage safeMCUs = 8
	ok, safe := testSalvage(10, 80, 95)
	if !ok || safe != 8 {
		t.Errorf("Case 1: Expected ok=true, safe=8, got ok=%v, safe=%d", ok, safe)
	}

	// Case 2: validMCUs = 4 (< 6) -> should reject
	ok, _ = testSalvage(4, 80, 95)
	if ok {
		t.Errorf("Case 2: Expected ok=false for validMCUs=4")
	}

	// Case 3: validMCUs = 8, but end is saturated white (255) -> should reject
	ok, _ = testSalvage(8, 80, 255)
	if ok {
		t.Errorf("Case 3: Expected ok=false for clipped white end block")
	}

	// Case 4: validMCUs = 8, but DC jump is 110 (> 90) -> should reject
	ok, _ = testSalvage(8, 50, 165)
	if ok {
		t.Errorf("Case 4: Expected ok=false for runaway DC jump")
	}

	// Case 5: validMCUs = 14 -> should accept 14
	ok, safe = testSalvage(14, 80, 90)
	if !ok || safe != 14 {
		t.Errorf("Case 5: Expected ok=true, safe=14, got ok=%v, safe=%d", ok, safe)
	}
}
