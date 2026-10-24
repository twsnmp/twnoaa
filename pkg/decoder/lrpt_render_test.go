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

	fullImg, chanA, chanB, dayFraction := renderLRPTImages(lineMap, lineKeys, 0)
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

	fullImg, chanA, chanB, dayFraction := renderLRPTImages(lineMap, lineKeys, 15000000)
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

	fullImg, _, _, dayFraction := renderLRPTImages(lineMap, lineKeys, 0)
	if fullImg == nil {
		t.Fatal("renderLRPTImages returned nil image")
	}

	if dayFraction < 0.3 || dayFraction > 0.7 {
		t.Errorf("Expected intermediate dayFraction ~0.5 for terminator pass, got %.3f", dayFraction)
	}
}
