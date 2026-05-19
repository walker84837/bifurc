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
	D, div := calculateDivergence(0, 0, 1000, 0, 0.85, 0.15, 1.0)
	if D != 0 {
		t.Errorf("D = %.4f, want 0", D)
	}
	if div != 0 {
		t.Errorf("divergence = %.2f, want 0", div)
	}
}

func TestCalculateDivergence_TextOnly(t *testing.T) {
	D, div := calculateDivergence(100, 0, 1000, 0, 0.85, 0.15, 1.0)
	wantD := 0.085
	if !approxEqual(D, wantD, 0.0001) {
		t.Errorf("D = %.4f, want %.4f", D, wantD)
	}
	if div <= 0 {
		t.Errorf("divergence should be > 0 for text-only change, got %.2f", div)
	}

	D2, div2 := calculateDivergence(100, 0, 1000, 0, 0.85, 0.15, 0.5)
	if D2 != D {
		t.Errorf("D should be same regardless of lambda: got D=%.4f, want D=%.4f", D2, D)
	}
	if div2 >= div {
		t.Errorf("lower lambda should give lower divergence: got %.2f, want < %.2f", div2, div)
	}
}

func TestCalculateDivergence_BinaryOnly(t *testing.T) {
	totalBin := int64(1024)
	deltaBin := int64(512)
	D, div := calculateDivergence(0, deltaBin, 0, totalBin, 0.85, 0.15, 1.0)
	wantD := (float64(deltaBin) / float64(totalBin)) * 0.15
	if !approxEqual(D, wantD, 0.0001) {
		t.Errorf("D = %.4f, want %.4f", D, wantD)
	}
	if div <= 0 {
		t.Errorf("divergence should be > 0, got %.2f", div)
	}
}

func TestCalculateDivergence_BothTextAndBinary(t *testing.T) {
	D, div := calculateDivergence(50, 256, 500, 1024, 0.85, 0.15, 1.0)
	wantD := (50.0/500.0)*0.85 + (256.0/1024.0)*0.15
	if !approxEqual(D, wantD, 0.0001) {
		t.Errorf("D = %.4f, want %.4f", D, wantD)
	}
	if div <= 0 {
		t.Errorf("divergence should be > 0, got %.2f", div)
	}
}

func TestCalculateDivergence_NoTextFiles(t *testing.T) {
	D, div := calculateDivergence(100, 0, 0, 1024, 0.85, 0.15, 1.0)
	// baseL=0, so text term skipped; no binary delta, so binary term contributes 0
	if D != 0 {
		t.Errorf("D = %.4f, want 0 (no text files, no binary delta)", D)
	}
	if div != 0 {
		t.Errorf("divergence = %.2f, want 0", div)
	}
}

func TestCalculateDivergence_NoBinaryFiles(t *testing.T) {
	D, div := calculateDivergence(100, 1024, 500, 0, 0.85, 0.15, 1.0)
	wantD := (100.0 / 500.0) * 0.85
	if !approxEqual(D, wantD, 0.0001) {
		t.Errorf("D = %.4f, want %.4f", D, wantD)
	}
	if div <= 0 {
		t.Errorf("divergence should be > 0, got %.2f", div)
	}
}

func TestCalculateDivergence_EmptyRepo(t *testing.T) {
	D, div := calculateDivergence(0, 0, 0, 0, 0.85, 0.15, 1.0)
	if D != 0 {
		t.Errorf("D = %.4f, want 0 for empty repo", D)
	}
	if div != 0 {
		t.Errorf("divergence = %.2f, want 0 for empty repo", div)
	}
}

func TestCalculateDivergence_LambdaEffect(t *testing.T) {
	_, div1 := calculateDivergence(200, 0, 1000, 0, 0.85, 0.15, 1.0)
	_, div2 := calculateDivergence(200, 0, 1000, 0, 0.85, 0.15, 2.0)
	_, div3 := calculateDivergence(200, 0, 1000, 0, 0.85, 0.15, 0.5)

	if div2 <= div1 {
		t.Errorf("higher lambda should give higher divergence: lambda=2 (%.2f) vs lambda=1 (%.2f)", div2, div1)
	}
	if div3 >= div1 {
		t.Errorf("lower lambda should give lower divergence: lambda=0.5 (%.2f) vs lambda=1 (%.2f)", div3, div1)
	}
}

func TestCalculateDivergence_Asymptotic(t *testing.T) {
	// D=5 with λ=1 → e^-5 ≈ 0.0067 → divergence ≈ 99.33%
	_, div := calculateDivergence(5000, 0, 1000, 0, 1.0, 0.0, 1.0)
	if div >= 100 {
		t.Errorf("divergence should approach 100 asymptotically, got %.2f", div)
	}
	if div < 99 {
		t.Errorf("D=5 should give divergence near 99%%, got %.2f", div)
	}
}

func TestCalculateDivergence_ZeroWeights(t *testing.T) {
	D, div := calculateDivergence(500, 1024, 1000, 2048, 0, 0, 1.0)
	if D != 0 {
		t.Errorf("D should be 0 with zero weights, got %.4f", D)
	}
	if div != 0 {
		t.Errorf("divergence should be 0 with zero weights, got %.2f", div)
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
