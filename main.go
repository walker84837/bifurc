package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	bifurc "github.com/walker84837/bifurc/pkg"

	"github.com/fatih/color"
)

type Stats struct {
	DeltaLines       int
	DeltaBinaryBytes int64
	AvgLoc           int
	AvgBinaryBytes   int64
	DivergenceText   float64
	DivergenceBinary float64
	Divergence       float64
	Lambda           float64
	WeightText       float64
	WeightBinary     float64
	Branch1          string
	Branch2          string
	Preset           string
	Sensitivity      string
	Format           string
	Separator        string
	NoColor          bool
	Detailed         bool
}

var (
	branch1      string
	branch2      string
	format       string
	separator    string
	noColor      bool
	detailed     bool
	preset       string
	sensitivity  string
	lambdaFlag   float64
	textWeight   float64
	binaryWeight float64
)

type presetWeights struct {
	weightText, weightBinary float64
}

var presets = map[string]presetWeights{
	"systems": {0.95, 0.05},
	"web":     {0.75, 0.25},
	"ml":      {0.60, 0.40},
	"cli":     {0.92, 0.08},
	"library": {0.88, 0.12},
	"custom":  {0.85, 0.15},
}

var sensitivityMult = map[string]float64{
	"low":    0.6,
	"normal": 1.0,
	"high":   1.5,
}

func init() {
	flag.StringVar(&branch1, "branch1", "", "First branch to compare (required)")
	flag.StringVar(&branch2, "branch2", "", "Second branch to compare (required)")
	flag.StringVar(&format, "format", "text", "Output format: text, json, custom")
	flag.StringVar(&separator, "separator", "\n", "Separator for custom output format")
	flag.BoolVar(&noColor, "no-color", false, "Disable color output")
	flag.BoolVar(&detailed, "detailed", false, "Show detailed component breakdown (text output only)")
	flag.StringVar(&preset, "preset", "custom", "Codebase type preset: systems, web, ml, cli, library, custom")
	flag.StringVar(&sensitivity, "sensitivity", "normal", "Curve sensitivity: low, normal, high")
	flag.Float64Var(&lambdaFlag, "lambda", 0, "Lambda override (0 = auto-calculate)")
	flag.Float64Var(&textWeight, "text-weight", 0.85, "Text weight (only with --preset custom)")
	flag.Float64Var(&binaryWeight, "binary-weight", 0.15, "Binary weight (only with --preset custom)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "bifurc - Compare Git branches and show divergence statistics\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s --branch1 main --branch2 feature-branch\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --branch1 HEAD~1 --branch2 HEAD --format json\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --branch1 develop --branch2 main --no-color\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if _, err := exec.LookPath("git"); err != nil {
		outputError("git is not installed or not in PATH", format)
	}

	if branch1 == "" || branch2 == "" {
		outputError("Both --branch1 and --branch2 are required", format)
	}

	gitClient := bifurc.GetGitClient()

	if !gitClient.CheckBranch(branch1) {
		outputError(fmt.Sprintf("Invalid branch1 '%s': branch does not exist", branch1), format)
	}
	if !gitClient.CheckBranch(branch2) {
		outputError(fmt.Sprintf("Invalid branch2 '%s': branch does not exist", branch2), format)
	}

	if format != "text" {
		color.NoColor = true
	} else {
		color.NoColor = noColor
	}

	if _, ok := presets[preset]; !ok {
		outputError(fmt.Sprintf("Invalid preset '%s': must be one of systems, web, ml, cli, library, custom", preset), format)
	}
	if _, ok := sensitivityMult[sensitivity]; !ok {
		outputError(fmt.Sprintf("Invalid sensitivity '%s': must be one of low, normal, high", sensitivity), format)
	}
	if lambdaFlag < 0 {
		outputError("Lambda must be >= 0", format)
	}

	pw := presets[preset]

	weightText := pw.weightText
	weightBinary := pw.weightBinary
	if preset == "custom" {
		weightText = textWeight
		weightBinary = binaryWeight
	}

	if math.Abs(weightText+weightBinary-1.0) > 1e-6 {
		outputError(fmt.Sprintf("Weights must sum to 1.0, got weightText=%.2f, weightBinary=%.2f (sum=%.4f)", weightText, weightBinary, weightText+weightBinary), format)
	}

	deltaLines, binaryFiles, err := gitClient.GetTextDiff(branch1, branch2)
	if err != nil {
		outputError(err.Error(), format)
	}

	var deltaBinaryBytes int64
	if len(binaryFiles) > 0 {
		deltaBinaryBytes, err = gitClient.GetBinaryByteDelta(branch1, branch2, binaryFiles)
		if err != nil {
			outputError(err.Error(), format)
		}
	}

	totalLoc1, totalBinBytes1, err := gitClient.GetRepoStats(branch1)
	if err != nil {
		outputError(err.Error(), format)
	}
	totalLoc2, totalBinBytes2, err := gitClient.GetRepoStats(branch2)
	if err != nil {
		outputError(err.Error(), format)
	}

	avgLoc := (totalLoc1 + totalLoc2) / 2
	avgBinaryBytes := (totalBinBytes1 + totalBinBytes2) / 2

	lambda := resolveLambda(lambdaFlag, sensitivity, avgLoc)
	divergenceText, divergenceBinary, divergence := calculateDivergence(deltaLines, deltaBinaryBytes, avgLoc, avgBinaryBytes, weightText, weightBinary)

	st := &Stats{
		DeltaLines:       deltaLines,
		DeltaBinaryBytes: deltaBinaryBytes,
		AvgLoc:           avgLoc,
		AvgBinaryBytes:   avgBinaryBytes,
		DivergenceText:   divergenceText,
		DivergenceBinary: divergenceBinary,
		Divergence:       divergence,
		Lambda:           lambda,
		WeightText:       weightText,
		WeightBinary:     weightBinary,
		Branch1:          branch1,
		Branch2:          branch2,
		Preset:           preset,
		Sensitivity:      sensitivity,
		Format:           format,
		Separator:        separator,
		NoColor:          noColor,
		Detailed:         detailed,
	}

	switch format {
	case "text":
		outputText(gitClient, st)
	case "json":
		outputJSON(st)
	case "custom":
		outputCustom(st)
	default:
		outputError("Invalid output format specified", format)
	}
}

const lambdaBase = 4.0

func autoLambda(loc int) float64 {
	if loc <= 0 {
		return 1.0
	}
	base := math.Log10(float64(loc))
	if base < 1 {
		base = 1
	}
	l := lambdaBase / base
	return math.Max(0.5, math.Min(2.0, l))
}

func resolveLambda(lambdaFlag float64, sensitivity string, baseL int) float64 {
	if lambdaFlag > 0 {
		return lambdaFlag
	}
	mult, ok := sensitivityMult[sensitivity]
	if !ok {
		mult = 1.0
	}
	return autoLambda(baseL) * mult
}

func calculateDivergence(deltaLines int, deltaBinaryBytes int64, avgLoc int, avgBinaryBytes int64, weightText, weightBinary float64) (divergenceText, divergenceBinary, divergence float64) {
	if avgLoc > 0 {
		divergenceText = (float64(deltaLines) / float64(avgLoc)) * weightText
	}
	if avgBinaryBytes > 0 {
		divergenceBinary = (float64(deltaBinaryBytes) / float64(avgBinaryBytes)) * weightBinary
	}
	divergence = divergenceText + divergenceBinary
	return
}

func divergenceImpact(divergence, lambda float64) float64 {
	return 100.0 * (1.0 - math.Exp(-lambda*divergence))
}

func formatBytes(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	case b < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	}
}

func outputText(gitClient *bifurc.GitClient, st *Stats) {
	if st.Format == "text" && !st.NoColor {
		if repoInfo, err := gitClient.GetRepoInfo(); err == nil {
			fmt.Printf("Repository: %s", color.CyanString(repoInfo))
			if st.Preset != "custom" {
				fmt.Printf(" (%s preset)", st.Preset)
			}
			fmt.Printf(" (λ=%.2f, sensitivity=%s)\n", st.Lambda, st.Sensitivity)
			fmt.Println()
		}
	}

	if st.Divergence < 0.001 && st.DeltaLines == 0 && st.DeltaBinaryBytes == 0 {
		color.Green("No changes between '%s' and '%s'.", st.Branch1, st.Branch2)
		return
	}

	fmt.Printf("  Total LOC:          %s\n", color.CyanString("%d", st.AvgLoc))
	fmt.Printf("  Total binary:       %s\n", color.CyanString(formatBytes(st.AvgBinaryBytes)))
	fmt.Printf("  Lines changed:      %s", color.YellowString("%d", st.DeltaLines))
	if st.AvgLoc > 0 {
		fmt.Printf("  (%s of LOC)", color.YellowString("%.1f%%", float64(st.DeltaLines)/float64(st.AvgLoc)*100))
	}
	fmt.Println()
	fmt.Printf("  Binary changed:     %s", color.YellowString(formatBytes(st.DeltaBinaryBytes)))
	if st.AvgBinaryBytes > 0 {
		fmt.Printf("  (%s of binaries)", color.YellowString("%.1f%%", float64(st.DeltaBinaryBytes)/float64(st.AvgBinaryBytes)*100))
	}
	fmt.Println()
	fmt.Printf("  Raw score D:        %s\n", color.CyanString("%.4f", st.Divergence))
	if st.Detailed {
		if st.Divergence > 0 {
			fmt.Printf("    Text component:   %s  (%s of D)\n",
				color.CyanString("%.4f", st.DivergenceText),
				color.YellowString("%.0f%%", st.DivergenceText/st.Divergence*100))
			fmt.Printf("    Binary component: %s  (%s of D)\n",
				color.CyanString("%.4f", st.DivergenceBinary),
				color.YellowString("%.0f%%", st.DivergenceBinary/st.Divergence*100))
		} else {
			fmt.Printf("    Text component:   %s\n", color.CyanString("%.4f", st.DivergenceText))
			fmt.Printf("    Binary component: %s\n", color.CyanString("%.4f", st.DivergenceBinary))
		}
	}
	fmt.Printf("  Divergence:          %s\n", color.GreenString("%.2f%%", st.Divergence*100))
	fmt.Printf("  Divergence Impact:   %s\n", color.GreenString("%.1f%%", divergenceImpact(st.Divergence, st.Lambda)))
}

func outputJSON(st *Stats) {
	out := struct {
		TotalLoc         int     `json:"totalLoc"`
		TotalBinaryBytes int64   `json:"totalBinaryBytes"`
		DeltaLines       int     `json:"deltaLines"`
		DeltaBinaryBytes int64   `json:"deltaBinaryBytes"`
		RawScore         float64 `json:"rawScore"`
		Lambda           float64 `json:"lambda"`
		Sensitivity      string  `json:"sensitivity"`
		Preset           string  `json:"preset"`
		TextWeight       float64 `json:"textWeight"`
		BinaryWeight     float64 `json:"binaryWeight"`
		Branch1          string  `json:"branch1"`
		Branch2          string  `json:"branch2"`
	}{
		TotalLoc:         st.AvgLoc,
		TotalBinaryBytes: st.AvgBinaryBytes,
		DeltaLines:       st.DeltaLines,
		DeltaBinaryBytes: st.DeltaBinaryBytes,
		RawScore:         math.Round(st.Divergence*10000) / 10000,
		Lambda:           math.Round(st.Lambda*100) / 100,
		Sensitivity:      st.Sensitivity,
		Preset:           st.Preset,
		TextWeight:       st.WeightText,
		BinaryWeight:     st.WeightBinary,
		Branch1:          st.Branch1,
		Branch2:          st.Branch2,
	}
	jsonData, err := json.Marshal(out)
	if err != nil {
		outputError(fmt.Sprintf("Error generating JSON: %v", err), st.Format)
	}
	fmt.Println(string(jsonData))
}

func outputCustom(st *Stats) {
	parts := []string{
		strconv.FormatFloat(st.Divergence*100, 'f', 2, 64),
		strconv.FormatFloat(st.Divergence, 'f', 4, 64),
		strconv.Itoa(st.DeltaLines),
		strconv.FormatInt(st.DeltaBinaryBytes, 10),
		strconv.Itoa(st.AvgLoc),
		strconv.FormatInt(st.AvgBinaryBytes, 10),
		strconv.FormatFloat(st.Lambda, 'f', 2, 64),
		strconv.FormatFloat(st.WeightText, 'f', 2, 64),
		strconv.FormatFloat(st.WeightBinary, 'f', 2, 64),
		st.Preset,
		st.Sensitivity,
		st.Branch1,
		st.Branch2,
	}
	fmt.Println(strings.Join(parts, st.Separator))
}

// outputError outputs an error message based on the configured format and exits the program with code 1.
func outputError(message string, format string) {
	switch format {
	case "json":
		errJSON := struct {
			Error string `json:"error"`
		}{
			Error: message,
		}
		jsonData, _ := json.Marshal(errJSON)
		fmt.Println(string(jsonData))
	default:
		color.Red(message)
	}
	os.Exit(1)
}
