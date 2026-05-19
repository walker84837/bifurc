package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	git "diffstat/pkg"

	"github.com/fatih/color"
)

var (
	branch1      string
	branch2      string
	format       string
	separator    string
	noColor      bool
	preset       string
	sensitivity  string
	lambdaFlag   float64
	textWeight   float64
	binaryWeight float64
)

type presetWeights struct {
	wt, wb float64
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
	flag.StringVar(&preset, "preset", "custom", "Codebase type preset: systems, web, ml, cli, library, custom")
	flag.StringVar(&sensitivity, "sensitivity", "normal", "Curve sensitivity: low, normal, high")
	flag.Float64Var(&lambdaFlag, "lambda", 0, "Lambda override (0 = auto-calculate)")
	flag.Float64Var(&textWeight, "text-weight", 0.85, "Text weight (only with --preset custom)")
	flag.Float64Var(&binaryWeight, "binary-weight", 0.15, "Binary weight (only with --preset custom)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "diffstat - Compare Git branches and show divergence statistics\n\n")
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

	if branch1 == "" || branch2 == "" {
		outputError("Both --branch1 and --branch2 are required")
	}

	gitClient := git.GetGitClient()

	if !gitClient.CheckBranch(branch1) {
		outputError(fmt.Sprintf("Invalid branch1 '%s': branch does not exist", branch1))
	}
	if !gitClient.CheckBranch(branch2) {
		outputError(fmt.Sprintf("Invalid branch2 '%s': branch does not exist", branch2))
	}

	if format != "text" {
		color.NoColor = true
	} else {
		color.NoColor = noColor
	}

	if _, ok := presets[preset]; !ok {
		outputError(fmt.Sprintf("Invalid preset '%s': must be one of systems, web, ml, cli, library, custom", preset))
	}
	if _, ok := sensitivityMult[sensitivity]; !ok {
		outputError(fmt.Sprintf("Invalid sensitivity '%s': must be one of low, normal, high", sensitivity))
	}
	if lambdaFlag < 0 {
		outputError("Lambda must be >= 0")
	}

	pw, ok := presets[preset]
	if !ok {
		pw = presets["custom"]
	}

	wt := pw.wt
	wb := pw.wb
	if preset == "custom" {
		wt = textWeight
		wb = binaryWeight
	}

	deltaLines, binaryFiles, err := gitClient.GetTextDiff(branch1, branch2)
	if err != nil {
		outputError(err.Error())
	}

	var deltaBinaryBytes int64
	if len(binaryFiles) > 0 {
		deltaBinaryBytes, err = gitClient.GetBinaryByteDelta(branch1, branch2, binaryFiles)
		if err != nil {
			outputError(err.Error())
		}
	}

	totalLoc1, totalBinBytes1, err := gitClient.GetRepoStats(branch1)
	if err != nil {
		outputError(err.Error())
	}
	totalLoc2, totalBinBytes2, err := gitClient.GetRepoStats(branch2)
	if err != nil {
		outputError(err.Error())
	}

	baseL := totalLoc1
	if totalLoc2 > totalLoc1 {
		baseL = totalLoc2
	}
	totalBinBytes := totalBinBytes1
	if totalBinBytes2 > totalBinBytes1 {
		totalBinBytes = totalBinBytes2
	}

	lambda := resolveLambda(lambdaFlag, sensitivity, baseL)
	D, divergence := calculateDivergence(deltaLines, deltaBinaryBytes, baseL, totalBinBytes, wt, wb, lambda)

	switch format {
	case "text":
		outputText(gitClient, deltaLines, deltaBinaryBytes, totalLoc1, totalLoc2, totalBinBytes1, totalBinBytes2, baseL, totalBinBytes, D, divergence, lambda, wt, wb)
	case "json":
		outputJSON(deltaLines, deltaBinaryBytes, baseL, totalBinBytes, D, divergence, lambda, wt, wb)
	case "custom":
		outputCustom(deltaLines, deltaBinaryBytes, baseL, totalBinBytes, D, divergence, lambda, wt, wb)
	default:
		outputError("Invalid output format specified")
	}
}

func autoLambda(loc int) float64 {
	if loc <= 0 {
		return 1.0
	}
	base := math.Log10(float64(loc))
	if base < 1 {
		base = 1
	}
	l := 4.0 / base
	return math.Max(0.5, math.Min(2.0, l))
}

func resolveLambda(lambdaFlag float64, sensitivity string, baseL int) float64 {
	if lambdaFlag > 0 {
		return lambdaFlag
	}
	return autoLambda(baseL) * sensitivityMult[sensitivity]
}

func calculateDivergence(deltaLines int, deltaBinaryBytes int64, baseL int, totalBinBytes int64, wt, wb, lambda float64) (D, divergence float64) {
	if baseL > 0 {
		D += (float64(deltaLines) / float64(baseL)) * wt
	}
	if totalBinBytes > 0 {
		D += (float64(deltaBinaryBytes) / float64(totalBinBytes)) * wb
	}
	divergence = 100.0 * (1.0 - math.Exp(-lambda*D))
	return D, divergence
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

func outputText(gitClient *git.GitClient, deltaLines int, deltaBinaryBytes int64, totalLoc1, totalLoc2 int, totalBinBytes1, totalBinBytes2 int64, baseL int, totalBinBytes int64, D, divergence, lambda, wt, wb float64) {
	if format == "text" && !noColor {
		if repoInfo, err := gitClient.GetRepoInfo(); err == nil {
			fmt.Printf("Repository: %s", color.CyanString(repoInfo))
			if preset != "custom" {
				fmt.Printf(" (%s preset)", preset)
			}
			fmt.Printf(" (λ=%.2f, sensitivity=%s)\n", lambda, sensitivity)
			fmt.Println()
		}
	}

	if divergence < 0.001 && deltaLines == 0 && deltaBinaryBytes == 0 {
		color.Green("No changes between '%s' and '%s'.", branch1, branch2)
		return
	}

	fmt.Printf("  Total LOC:          %s\n", color.CyanString("%d", baseL))
	fmt.Printf("  Total binary:       %s\n", color.CyanString(formatBytes(totalBinBytes)))
	fmt.Printf("  Lines changed:      %s", color.YellowString("%d", deltaLines))
	if baseL > 0 {
		fmt.Printf("  (%s of LOC)", color.YellowString("%.1f%%", float64(deltaLines)/float64(baseL)*100))
	}
	fmt.Println()
	fmt.Printf("  Binary changed:     %s", color.YellowString(formatBytes(deltaBinaryBytes)))
	if totalBinBytes > 0 {
		fmt.Printf("  (%s of binaries)", color.YellowString("%.1f%%", float64(deltaBinaryBytes)/float64(totalBinBytes)*100))
	}
	fmt.Println()
	fmt.Printf("  Raw score D:        %s\n", color.CyanString("%.4f", D))
	fmt.Printf("  Divergence:         %s\n", color.GreenString("%.1f%%", divergence))
}

func outputJSON(deltaLines int, deltaBinaryBytes int64, baseL int, totalBinBytes int64, D, divergence, lambda, wt, wb float64) {
	out := struct {
		TotalLoc         int     `json:"totalLoc"`
		TotalBinaryBytes int64   `json:"totalBinaryBytes"`
		DeltaLines       int     `json:"deltaLines"`
		DeltaBinaryBytes int64   `json:"deltaBinaryBytes"`
		RawScore         float64 `json:"rawScore"`
		DivergencePct    float64 `json:"divergencePercent"`
		Lambda           float64 `json:"lambda"`
		Sensitivity      string  `json:"sensitivity"`
		Preset           string  `json:"preset"`
		TextWeight       float64 `json:"textWeight"`
		BinaryWeight     float64 `json:"binaryWeight"`
		Branch1          string  `json:"branch1"`
		Branch2          string  `json:"branch2"`
	}{
		TotalLoc:         baseL,
		TotalBinaryBytes: totalBinBytes,
		DeltaLines:       deltaLines,
		DeltaBinaryBytes: deltaBinaryBytes,
		RawScore:         math.Round(D*10000) / 10000,
		DivergencePct:    math.Round(divergence*100) / 100,
		Lambda:           math.Round(lambda*100) / 100,
		Sensitivity:      sensitivity,
		Preset:           preset,
		TextWeight:       wt,
		BinaryWeight:     wb,
		Branch1:          branch1,
		Branch2:          branch2,
	}
	jsonData, err := json.Marshal(out)
	if err != nil {
		outputError(fmt.Sprintf("Error generating JSON: %v", err))
	}
	fmt.Println(string(jsonData))
}

func outputCustom(deltaLines int, deltaBinaryBytes int64, baseL int, totalBinBytes int64, D, divergence, lambda, wt, wb float64) {
	parts := []string{
		strconv.FormatFloat(divergence, 'f', 2, 64),
		strconv.FormatFloat(D, 'f', 4, 64),
		strconv.Itoa(deltaLines),
		strconv.FormatInt(deltaBinaryBytes, 10),
		strconv.Itoa(baseL),
		strconv.FormatInt(totalBinBytes, 10),
		strconv.FormatFloat(lambda, 'f', 2, 64),
		strconv.FormatFloat(wt, 'f', 2, 64),
		strconv.FormatFloat(wb, 'f', 2, 64),
		preset,
		sensitivity,
		branch1,
		branch2,
	}
	fmt.Println(strings.Join(parts, separator))
}

func outputError(message string) {
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
