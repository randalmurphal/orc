package bench

import (
	"math"
	"math/rand"
	"sort"
)

// ConfidenceInterval holds a bootstrap confidence interval.
type ConfidenceInterval struct {
	Lower      float64 `json:"lower"`
	Upper      float64 `json:"upper"`
	Median     float64 `json:"median"`
	Confidence float64 `json:"confidence"` // e.g. 0.95
}

// PairedComparison holds the result of comparing two variants.
type PairedComparison struct {
	VariantA     string             `json:"variant_a"`
	VariantB     string             `json:"variant_b"`
	MeanDiff     float64            `json:"mean_diff"`     // A - B (positive = A is better)
	PValue       float64            `json:"p_value"`       // Two-sided
	Significant  bool               `json:"significant"`   // p < alpha
	EffectSize   float64            `json:"effect_size"`   // Cohen's d
	CI           ConfidenceInterval `json:"ci"`            // Bootstrap CI of the difference
	TestUsed     string             `json:"test_used"`     // "wilcoxon" or "mcnemar"
	SampleSize   int                `json:"sample_size"`
}

// BootstrapCI computes a BCa (bias-corrected and accelerated) bootstrap
// confidence interval for the mean of the given samples.
func BootstrapCI(samples []float64, confidence float64, nBootstrap int) ConfidenceInterval {
	if len(samples) == 0 {
		return ConfidenceInterval{Confidence: confidence}
	}
	if len(samples) == 1 {
		return ConfidenceInterval{
			Lower:      samples[0],
			Upper:      samples[0],
			Median:     samples[0],
			Confidence: confidence,
		}
	}

	rng := rand.New(rand.NewSource(42)) // Deterministic for reproducibility

	// Compute observed statistic
	observed := mean(samples)

	// Generate bootstrap distribution
	bootMeans := make([]float64, nBootstrap)
	n := len(samples)
	for b := 0; b < nBootstrap; b++ {
		sum := 0.0
		for i := 0; i < n; i++ {
			sum += samples[rng.Intn(n)]
		}
		bootMeans[b] = sum / float64(n)
	}
	sort.Float64s(bootMeans)

	// BCa correction: bias correction factor
	countBelow := 0
	for _, bm := range bootMeans {
		if bm < observed {
			countBelow++
		}
	}
	z0 := normalQuantile(float64(countBelow) / float64(nBootstrap))

	// Acceleration factor (jackknife)
	jackMeans := make([]float64, n)
	for i := 0; i < n; i++ {
		sum := 0.0
		count := 0
		for j := 0; j < n; j++ {
			if j != i {
				sum += samples[j]
				count++
			}
		}
		jackMeans[i] = sum / float64(count)
	}
	jackMean := mean(jackMeans)

	var numSum, denSum float64
	for _, jm := range jackMeans {
		diff := jackMean - jm
		numSum += diff * diff * diff
		denSum += diff * diff
	}

	var acc float64
	if denSum > 0 {
		acc = numSum / (6 * math.Pow(denSum, 1.5))
	}

	// Adjusted percentiles
	alpha := (1 - confidence) / 2
	zAlpha := normalQuantile(alpha)
	zUpper := normalQuantile(1 - alpha)

	a1 := normalCDF(z0 + (z0+zAlpha)/(1-acc*(z0+zAlpha)))
	a2 := normalCDF(z0 + (z0+zUpper)/(1-acc*(z0+zUpper)))

	// Clamp to valid indices
	lowerIdx := clampIdx(int(a1*float64(nBootstrap)), nBootstrap)
	upperIdx := clampIdx(int(a2*float64(nBootstrap)), nBootstrap)
	medianIdx := clampIdx(nBootstrap/2, nBootstrap)

	return ConfidenceInterval{
		Lower:      bootMeans[lowerIdx],
		Upper:      bootMeans[upperIdx],
		Median:     bootMeans[medianIdx],
		Confidence: confidence,
	}
}

// WilcoxonSignedRank performs a Wilcoxon signed-rank test on paired samples.
// Returns the p-value (two-sided) for the null hypothesis that the median
// difference is zero. Uses normal approximation for n > 20.
func WilcoxonSignedRank(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 1.0
	}

	// Compute differences, ignoring zeros
	type ranked struct {
		absVal float64
		sign   float64
	}

	var diffs []ranked
	for i := range a {
		d := a[i] - b[i]
		if d == 0 {
			continue
		}
		sign := 1.0
		if d < 0 {
			sign = -1.0
		}
		diffs = append(diffs, ranked{absVal: math.Abs(d), sign: sign})
	}

	n := len(diffs)
	if n == 0 {
		return 1.0
	}

	// Rank by absolute value
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].absVal < diffs[j].absVal
	})

	// Assign ranks (average ties)
	ranks := make([]float64, n)
	i := 0
	for i < n {
		j := i
		for j < n && diffs[j].absVal == diffs[i].absVal {
			j++
		}
		avgRank := float64(i+j+1) / 2.0
		for k := i; k < j; k++ {
			ranks[k] = avgRank
		}
		i = j
	}

	// Compute W+ (sum of positive ranks)
	var wPlus float64
	for i, d := range diffs {
		if d.sign > 0 {
			wPlus += ranks[i]
		}
	}

	// Normal approximation (valid for n > ~10)
	nf := float64(n)
	expectedW := nf * (nf + 1) / 4
	varW := nf * (nf + 1) * (2*nf + 1) / 24

	z := (wPlus - expectedW) / math.Sqrt(varW)
	// Two-sided p-value
	p := 2 * normalCDF(-math.Abs(z))

	return math.Min(p, 1.0)
}

// McNemarTest performs McNemar's test on paired binary outcomes.
// a and b are slices of 0/1 values (fail/pass) for the same tasks.
// Returns the p-value for the null hypothesis of no difference.
func McNemarTest(a, b []int) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 1.0
	}

	// Count discordant pairs
	var b01, c10 float64 // b: A fails, B passes; c: A passes, B fails
	for i := range a {
		if a[i] == 0 && b[i] == 1 {
			b01++
		} else if a[i] == 1 && b[i] == 0 {
			c10++
		}
	}

	total := b01 + c10
	if total == 0 {
		return 1.0
	}

	// McNemar's chi-squared with Yates continuity correction.
	// The correction on |b - c| is 1.0 (not 0.5), per Agresti (2002).
	diff := math.Abs(b01-c10) - 1.0
	if diff < 0 {
		diff = 0
	}
	chi2 := diff * diff / total

	// p-value from chi-squared distribution with df=1
	p := 1 - chi2CDF(chi2)
	return math.Min(math.Max(p, 0), 1.0)
}

// PairedCohensD computes Cohen's d_z for paired samples (the correct
// effect size for repeated-measures designs like benchmark comparisons).
// d_z = mean(differences) / SD(differences)
func PairedCohensD(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) || len(a) < 2 {
		return 0
	}

	diffs := make([]float64, len(a))
	for i := range a {
		diffs[i] = a[i] - b[i]
	}

	m := mean(diffs)
	sd := math.Sqrt(variance(diffs, m))
	if sd == 0 || math.IsNaN(sd) {
		return 0
	}
	return m / sd
}

// ComparePaired performs a full paired comparison between two variants.
// scores are maps of variantID → []float64 (one score per task).
// binary are maps of variantID → []int (0/1 pass/fail per task).
func ComparePaired(variantA, variantB string, scoresA, scoresB []float64, binaryA, binaryB []int, alpha float64) PairedComparison {
	result := PairedComparison{
		VariantA:   variantA,
		VariantB:   variantB,
		SampleSize: len(scoresA),
	}

	if len(scoresA) > 0 && len(scoresA) == len(scoresB) {
		// Continuous comparison
		result.MeanDiff = mean(scoresA) - mean(scoresB)
		result.PValue = WilcoxonSignedRank(scoresA, scoresB)
		result.EffectSize = PairedCohensD(scoresA, scoresB)
		result.TestUsed = "wilcoxon"

		// Bootstrap CI on the differences
		diffs := make([]float64, len(scoresA))
		for i := range scoresA {
			diffs[i] = scoresA[i] - scoresB[i]
		}
		result.CI = BootstrapCI(diffs, 0.95, 10000)
	} else if len(binaryA) > 0 && len(binaryA) == len(binaryB) {
		// Binary comparison (pass/fail)
		result.PValue = McNemarTest(binaryA, binaryB)
		result.TestUsed = "mcnemar"

		// Mean difference as pass rate difference
		result.MeanDiff = binaryMean(binaryA) - binaryMean(binaryB)
	}

	result.Significant = result.PValue < alpha

	return result
}

// --- Utility functions ---

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func variance(vals []float64, m float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		diff := v - m
		sum += diff * diff
	}
	return sum / float64(len(vals)-1)
}

func binaryMean(vals []int) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += v
	}
	return float64(sum) / float64(len(vals))
}

func clampIdx(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// normalCDF computes the CDF of the standard normal distribution.
func normalCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

// normalQuantile computes the quantile function (inverse CDF) of the standard normal.
// Uses math.Erfinv for machine-precision accuracy.
func normalQuantile(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}
	return math.Sqrt2 * math.Erfinv(2*p-1)
}

// chi2CDF computes the CDF of chi-squared distribution with df=1.
// Uses the relationship: chi2(1) CDF = 2 * normalCDF(sqrt(x)) - 1
func chi2CDF(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return 2*normalCDF(math.Sqrt(x)) - 1
}
