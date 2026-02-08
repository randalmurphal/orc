package bench

import (
	"math"
	"testing"
)

func TestBootstrapCI(t *testing.T) {
	// Known dataset: 100 values from N(10, 2)
	samples := make([]float64, 100)
	for i := range samples {
		// Deterministic "normal-ish" data centered around 10
		samples[i] = 10 + 2*math.Sin(float64(i)*0.5)
	}

	ci := BootstrapCI(samples, 0.95, 10000)

	if ci.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", ci.Confidence)
	}
	if ci.Lower >= ci.Upper {
		t.Errorf("lower (%f) should be less than upper (%f)", ci.Lower, ci.Upper)
	}
	if ci.Median < ci.Lower || ci.Median > ci.Upper {
		t.Errorf("median (%f) should be between lower (%f) and upper (%f)", ci.Median, ci.Lower, ci.Upper)
	}

	// The mean should be near 10
	m := mean(samples)
	if m < 9 || m > 11 {
		t.Errorf("expected mean near 10, got %f", m)
	}
	// CI should contain the true mean
	if ci.Lower > m || ci.Upper < m {
		t.Errorf("CI [%f, %f] should contain mean %f", ci.Lower, ci.Upper, m)
	}
}

func TestBootstrapCI_SingleSample(t *testing.T) {
	ci := BootstrapCI([]float64{42.0}, 0.95, 1000)
	if ci.Lower != 42.0 || ci.Upper != 42.0 || ci.Median != 42.0 {
		t.Errorf("single sample CI should be [42, 42], got [%f, %f]", ci.Lower, ci.Upper)
	}
}

func TestBootstrapCI_Empty(t *testing.T) {
	ci := BootstrapCI(nil, 0.95, 1000)
	if ci.Lower != 0 || ci.Upper != 0 {
		t.Errorf("empty CI should be [0, 0], got [%f, %f]", ci.Lower, ci.Upper)
	}
}

func TestWilcoxonSignedRank(t *testing.T) {
	tests := []struct {
		name    string
		a, b    []float64
		wantSig bool // expect p < 0.05?
	}{
		{
			name:    "identical",
			a:       []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			b:       []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			wantSig: false,
		},
		{
			name:    "clearly different",
			a:       []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			b:       []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			wantSig: true,
		},
		{
			name:    "small shift",
			a:       []float64{1.1, 2.1, 3.1, 4.1, 5.1, 6.1, 7.1, 8.1, 9.1, 10.1},
			b:       []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			wantSig: true, // Small but consistent shift should be significant with n=10
		},
		{
			name:    "empty",
			a:       nil,
			b:       nil,
			wantSig: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := WilcoxonSignedRank(tt.a, tt.b)
			if p < 0 || p > 1 {
				t.Errorf("p-value should be in [0, 1], got %f", p)
			}
			got := p < 0.05
			if got != tt.wantSig {
				t.Errorf("significant=%v (p=%f), want significant=%v", got, p, tt.wantSig)
			}
		})
	}
}

func TestMcNemarTest(t *testing.T) {
	tests := []struct {
		name    string
		a, b    []int
		wantSig bool
	}{
		{
			name:    "identical outcomes",
			a:       []int{1, 1, 0, 0, 1, 1, 0, 0, 1, 1},
			b:       []int{1, 1, 0, 0, 1, 1, 0, 0, 1, 1},
			wantSig: false,
		},
		{
			name: "clearly different",
			a:    []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			b:    []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantSig: true,
		},
		{
			name:    "empty",
			a:       nil,
			b:       nil,
			wantSig: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := McNemarTest(tt.a, tt.b)
			if p < 0 || p > 1 {
				t.Errorf("p-value should be in [0, 1], got %f", p)
			}
			got := p < 0.05
			if got != tt.wantSig {
				t.Errorf("significant=%v (p=%f), want significant=%v", got, p, tt.wantSig)
			}
		})
	}
}

func TestPairedCohensD(t *testing.T) {
	// Identical samples → d=0 (all differences are 0)
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{1, 2, 3, 4, 5}
	d := PairedCohensD(a, b)
	if d != 0 {
		t.Errorf("identical samples should have d=0, got %f", d)
	}

	// Large consistent shift → large d_z (differences all = 9, SD = 0)
	// Since SD of differences is 0, returns 0 (can't compute)
	a = []float64{10, 11, 12, 13, 14}
	b = []float64{1, 2, 3, 4, 5}
	d = PairedCohensD(a, b)
	// All diffs = 9 exactly, so SD(diffs) = 0 → d = 0 (guarded)
	if d != 0 {
		t.Errorf("constant differences should have d=0 (undefined), got %f", d)
	}

	// Variable shift → meaningful d_z
	a = []float64{10, 22, 30, 42, 50}
	b = []float64{1, 2, 3, 4, 5}
	d = PairedCohensD(a, b)
	if d < 1 {
		t.Errorf("expected large paired effect size, got %f", d)
	}

	// Empty
	d = PairedCohensD(nil, nil)
	if d != 0 {
		t.Errorf("empty samples should have d=0, got %f", d)
	}

	// Single sample → insufficient data
	d = PairedCohensD([]float64{5}, []float64{10})
	if d != 0 {
		t.Errorf("single sample should have d=0, got %f", d)
	}
}

func TestComparePaired(t *testing.T) {
	scoresA := []float64{10, 20, 30, 40, 50}
	scoresB := []float64{5, 10, 15, 20, 25}
	binaryA := []int{1, 1, 1, 0, 1}
	binaryB := []int{0, 1, 0, 0, 1}

	result := ComparePaired("A", "B", scoresA, scoresB, binaryA, binaryB, 0.05)

	if result.VariantA != "A" || result.VariantB != "B" {
		t.Errorf("wrong variant labels")
	}
	if result.MeanDiff <= 0 {
		t.Errorf("expected positive mean diff (A > B), got %f", result.MeanDiff)
	}
	if result.TestUsed != "wilcoxon" {
		t.Errorf("expected wilcoxon test, got %s", result.TestUsed)
	}
	if result.SampleSize != 5 {
		t.Errorf("expected sample size 5, got %d", result.SampleSize)
	}
}

func TestNormalCDF(t *testing.T) {
	// CDF at 0 should be 0.5
	if v := normalCDF(0); math.Abs(v-0.5) > 0.001 {
		t.Errorf("normalCDF(0) = %f, want ~0.5", v)
	}

	// CDF at very negative should approach 0
	if v := normalCDF(-5); v > 0.001 {
		t.Errorf("normalCDF(-5) = %f, want ~0", v)
	}

	// CDF at very positive should approach 1
	if v := normalCDF(5); v < 0.999 {
		t.Errorf("normalCDF(5) = %f, want ~1", v)
	}
}
