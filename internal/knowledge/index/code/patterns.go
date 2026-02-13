package code

import (
	"fmt"
	"sort"
)

const (
	similarityThreshold = 0.80
	minClusterSize      = 5
)

// PatternDetector detects structural patterns across files.
type PatternDetector struct{}

// NewPatternDetector creates a new pattern detector.
func NewPatternDetector() *PatternDetector {
	return &PatternDetector{}
}

// Detect clusters structurally similar files into named patterns.
func (d *PatternDetector) Detect(files map[string][]Symbol) ([]Pattern, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// Filter out files with no symbols (unparseable)
	validFiles := make(map[string][]Symbol)
	for path, syms := range files {
		if len(syms) > 0 {
			validFiles[path] = syms
		}
	}

	if len(validFiles) < minClusterSize {
		return nil, nil
	}

	// Compute structural fingerprints for each file
	fingerprints := make(map[string]structFingerprint)
	for path, syms := range validFiles {
		fingerprints[path] = computeFingerprint(syms)
	}

	// Cluster files by similarity
	paths := make([]string, 0, len(fingerprints))
	for p := range fingerprints {
		paths = append(paths, p)
	}
	sort.Strings(paths) // deterministic ordering

	clustered := make(map[string]bool)
	var patterns []Pattern

	for i, p1 := range paths {
		if clustered[p1] {
			continue
		}

		cluster := []string{p1}
		for j := i + 1; j < len(paths); j++ {
			p2 := paths[j]
			if clustered[p2] {
				continue
			}
			sim := similarity(fingerprints[p1], fingerprints[p2])
			if sim >= similarityThreshold {
				cluster = append(cluster, p2)
			}
		}

		if len(cluster) >= minClusterSize {
			for _, p := range cluster {
				clustered[p] = true
			}
			patterns = append(patterns, Pattern{
				Name:          fmt.Sprintf("pattern_%d", len(patterns)+1),
				MemberCount:   len(cluster),
				CanonicalFile: cluster[0],
				Members:       cluster,
			})
		}
	}

	return patterns, nil
}

// structFingerprint captures the structural shape of a file's symbols.
type structFingerprint struct {
	kindCounts map[SymbolKind]int
	totalCount int
	hasParent  bool
	kindSeq    []SymbolKind // ordered sequence of symbol kinds
}

func computeFingerprint(symbols []Symbol) structFingerprint {
	fp := structFingerprint{
		kindCounts: make(map[SymbolKind]int),
	}

	for _, s := range symbols {
		fp.kindCounts[s.Kind]++
		fp.totalCount++
		if s.Parent != "" {
			fp.hasParent = true
		}
		fp.kindSeq = append(fp.kindSeq, s.Kind)
	}

	return fp
}

// similarity computes structural similarity between two fingerprints.
// Returns a value between 0.0 and 1.0.
func similarity(a, b structFingerprint) float64 {
	if a.totalCount == 0 && b.totalCount == 0 {
		return 1.0
	}
	if a.totalCount == 0 || b.totalCount == 0 {
		return 0.0
	}

	// Compare kind count distributions
	allKinds := make(map[SymbolKind]bool)
	for k := range a.kindCounts {
		allKinds[k] = true
	}
	for k := range b.kindCounts {
		allKinds[k] = true
	}

	matchScore := 0.0
	totalKinds := float64(len(allKinds))
	if totalKinds == 0 {
		return 0.0
	}

	for kind := range allKinds {
		countA := a.kindCounts[kind]
		countB := b.kindCounts[kind]
		if countA == 0 && countB == 0 {
			matchScore += 1.0
			continue
		}
		if countA == 0 || countB == 0 {
			continue
		}
		// Compare counts — penalize differences
		min, max := countA, countB
		if min > max {
			min, max = max, min
		}
		matchScore += float64(min) / float64(max)
	}

	kindSim := matchScore / totalKinds

	// Compare sequence lengths
	seqSim := 0.0
	if a.totalCount == b.totalCount {
		seqSim = 1.0
	} else {
		minTotal := a.totalCount
		maxTotal := b.totalCount
		if minTotal > maxTotal {
			minTotal, maxTotal = maxTotal, minTotal
		}
		seqSim = float64(minTotal) / float64(maxTotal)
	}

	// Compare parent structure
	parentSim := 0.0
	if a.hasParent == b.hasParent {
		parentSim = 1.0
	}

	// Weighted combination
	return kindSim*0.5 + seqSim*0.3 + parentSim*0.2
}
