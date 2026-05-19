package main

import (
	"testing"
)

func TestAutoLambda(t *testing.T) {
	tests := []struct {
		loc  int
		want float64
	}{
		{0, 1.0},
		{1, 2.0},
		{10, 2.0},
		{100, 2.0},
		{1000, 1.33},
		{10000, 1.0},
		{100000, 0.8},
		{1000000, 0.67},
	}

	for _, tt := range tests {
		got := autoLambda(tt.loc)
		if !approxEqual(got, tt.want, 0.01) {
			t.Errorf("autoLambda(%d) = %.2f, want %.2f", tt.loc, got, tt.want)
		}
	}
}

func TestCalculateDivergence_NoChanges(t *testing.T) {
	_, _, divergence := calculateDivergence(0, 0, 1000, 0, 0.85, 0.15)
	if divergence != 0 {
		t.Errorf("divergence = %.4f, want 0", divergence)
	}
}

func TestCalculateDivergence_TextOnly(t *testing.T) {
	divergenceText, divergenceBinary, divergence := calculateDivergence(100, 0, 1000, 0, 0.85, 0.15)
	wantDivergence := 0.085
	if !approxEqual(divergence, wantDivergence, 0.0001) {
		t.Errorf("divergence = %.4f, want %.4f", divergence, wantDivergence)
	}
	if !approxEqual(divergenceText, wantDivergence, 0.0001) {
		t.Errorf("divergenceText = %.4f, want %.4f", divergenceText, wantDivergence)
	}
	if divergenceBinary != 0 {
		t.Errorf("divergenceBinary = %.4f, want 0", divergenceBinary)
	}
}

func TestCalculateDivergence_BinaryOnly(t *testing.T) {
	totalBin := int64(1024)
	deltaBin := int64(512)
	divergenceText, divergenceBinary, divergence := calculateDivergence(0, deltaBin, 0, totalBin, 0.85, 0.15)
	wantDivergence := (float64(deltaBin) / float64(totalBin)) * 0.15
	if !approxEqual(divergence, wantDivergence, 0.0001) {
		t.Errorf("divergence = %.4f, want %.4f", divergence, wantDivergence)
	}
	if !approxEqual(divergenceBinary, wantDivergence, 0.0001) {
		t.Errorf("divergenceBinary = %.4f, want %.4f", divergenceBinary, wantDivergence)
	}
	if divergenceText != 0 {
		t.Errorf("divergenceText = %.4f, want 0", divergenceText)
	}
}

func TestCalculateDivergence_BothTextAndBinary(t *testing.T) {
	divergenceText, divergenceBinary, divergence := calculateDivergence(50, 256, 500, 1024, 0.85, 0.15)
	wantDivergenceText := (50.0 / 500.0) * 0.85
	wantDivergenceBinary := (256.0 / 1024.0) * 0.15
	wantDivergence := wantDivergenceText + wantDivergenceBinary
	if !approxEqual(divergenceText, wantDivergenceText, 0.0001) {
		t.Errorf("divergenceText = %.4f, want %.4f", divergenceText, wantDivergenceText)
	}
	if !approxEqual(divergenceBinary, wantDivergenceBinary, 0.0001) {
		t.Errorf("divergenceBinary = %.4f, want %.4f", divergenceBinary, wantDivergenceBinary)
	}
	if !approxEqual(divergence, wantDivergence, 0.0001) {
		t.Errorf("divergence = %.4f, want %.4f", divergence, wantDivergence)
	}
}

func TestCalculateDivergence_NoTextFiles(t *testing.T) {
	_, _, divergence := calculateDivergence(100, 0, 0, 1024, 0.85, 0.15)
	// avgLoc=0, so text term skipped; no binary delta, so binary term contributes 0
	if divergence != 0 {
		t.Errorf("divergence = %.4f, want 0 (no text files, no binary delta)", divergence)
	}
}

func TestCalculateDivergence_NoBinaryFiles(t *testing.T) {
	_, _, divergence := calculateDivergence(100, 1024, 500, 0, 0.85, 0.15)
	wantDivergence := (100.0 / 500.0) * 0.85
	if !approxEqual(divergence, wantDivergence, 0.0001) {
		t.Errorf("divergence = %.4f, want %.4f", divergence, wantDivergence)
	}
}

func TestCalculateDivergence_EmptyRepo(t *testing.T) {
	_, _, divergence := calculateDivergence(0, 0, 0, 0, 0.85, 0.15)
	if divergence != 0 {
		t.Errorf("divergence = %.4f, want 0 for empty repo", divergence)
	}
}

func TestCalculateDivergence_ZeroWeights(t *testing.T) {
	_, _, divergence := calculateDivergence(500, 1024, 1000, 2048, 0, 0)
	if divergence != 0 {
		t.Errorf("divergence should be 0 with zero weights, got %.4f", divergence)
	}
}

func TestDivergenceImpact(t *testing.T) {
	di1 := divergenceImpact(0.17, 1.0)
	di2 := divergenceImpact(0.17, 2.0)
	di3 := divergenceImpact(0.17, 0.5)

	if di2 <= di1 {
		t.Errorf("higher lambda should give higher impact: lambda=2 (%.2f) vs lambda=1 (%.2f)", di2, di1)
	}
	if di3 >= di1 {
		t.Errorf("lower lambda should give lower impact: lambda=0.5 (%.2f) vs lambda=1 (%.2f)", di3, di1)
	}
}

func TestDivergenceImpact_Asymptotic(t *testing.T) {
	// D=5 with λ=1 → e^-5 ≈ 0.0067 → divergence impact ≈ 99.33%
	di := divergenceImpact(5.0, 1.0)
	if di >= 100 {
		t.Errorf("divergence impact should approach 100 asymptotically, got %.2f", di)
	}
	if di < 99 {
		t.Errorf("D=5 should give divergence impact near 99%%, got %.2f", di)
	}
}

func TestResolveLambda_ManualOverride(t *testing.T) {
	l := resolveLambda(1.5, "normal", 10000)
	if l != 1.5 {
		t.Errorf("manual lambda override: got %.2f, want 1.5", l)
	}
}

func TestResolveLambda_AutoWithSensitivity(t *testing.T) {
	tests := []struct {
		sensitivity string
		baseL       int
		want        float64
	}{
		{"low", 10000, 0.6},
		{"normal", 10000, 1.0},
		{"high", 10000, 1.5},
		{"low", 100, 1.2},
		{"normal", 100, 2.0},
		{"high", 100, 3.0},
	}

	for _, tt := range tests {
		got := resolveLambda(0, tt.sensitivity, tt.baseL)
		if !approxEqual(got, tt.want, 0.01) {
			t.Errorf("resolveLambda(0, %q, %d) = %.2f, want %.2f", tt.sensitivity, tt.baseL, got, tt.want)
		}
	}
}

func approxEqual(a, b, tol float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= tol
}
