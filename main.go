package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	git "diffstat/pkg"

	"github.com/fatih/color"
)

var (
	branch1   string
	branch2   string
	format    string
	separator string
	noColor   bool
)

func init() {
	flag.StringVar(&branch1, "branch1", "", "First branch to compare (required)")
	flag.StringVar(&branch2, "branch2", "", "Second branch to compare (required)")
	flag.StringVar(&format, "format", "text", "Output format: text, json, custom")
	flag.StringVar(&separator, "separator", "\n", "Separator for custom output format")
	flag.BoolVar(&noColor, "no-color", false, "Disable color output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "diffstat - Compare Git branches and show statistics\n\n")
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

	// Validate branches exist
	if !gitClient.CheckBranch(branch1) {
		outputError(fmt.Sprintf("Invalid branch1 '%s': branch does not exist", branch1))
	}
	if !gitClient.CheckBranch(branch2) {
		outputError(fmt.Sprintf("Invalid branch2 '%s': branch does not exist", branch2))
	}

	// Configure color output
	if format != "text" {
		color.NoColor = true
	} else {
		color.NoColor = noColor
	}

	// Show repository info for text format
	if format == "text" && !noColor {
		if repoInfo, err := gitClient.GetRepoInfo(); err == nil {
			fmt.Printf("Repository: %s\n", color.CyanString(repoInfo))
			fmt.Println()
		}
	}

	showProgress := format == "text" && !noColor
	totalLines, err := gitClient.GetTotalLinesWithProgress(showProgress)
	if err != nil {
		outputError(fmt.Sprintf("Failed to count total lines: %v\nMake sure you're in a Git repository and git is installed.", err))
	}

	changedLines, err := gitClient.GetChangedLines(branch1, branch2)
	if err != nil {
		outputError(fmt.Sprintf("Failed to compare branches '%s' and '%s': %v\nCheck if the branches exist and are accessible.", branch1, branch2, err))
	}

	if changedLines == 0 && format == "text" {
		color.Green("No changes between the branches.")
		return
	}

	var percentageChange float64
	if totalLines > 0 {
		percentageChange = float64(changedLines) / float64(totalLines) * 100
	}

	switch format {
	case "text":
		fmt.Printf("Total lines in repository: %s\n", color.CyanString("%d", totalLines))
		fmt.Printf("Lines changed between %s and %s: %s\n", branch1, branch2, color.YellowString("%d", changedLines))
		fmt.Printf("Percentage of change: %s\n", color.GreenString("%.2f%%", percentageChange))
	case "json":
		out := struct {
			TotalLines   int     `json:"totalLines"`
			ChangedLines int     `json:"changedLines"`
			Percentage   float64 `json:"percentage"`
			Branch1      string  `json:"branch1"`
			Branch2      string  `json:"branch2"`
		}{
			TotalLines:   totalLines,
			ChangedLines: changedLines,
			Percentage:   percentageChange,
			Branch1:      branch1,
			Branch2:      branch2,
		}
		jsonData, err := json.Marshal(out)
		if err != nil {
			outputError(fmt.Sprintf("Error generating JSON: %v", err))
		}
		fmt.Println(string(jsonData))
	case "custom":
		parts := []string{
			strconv.Itoa(totalLines),
			strconv.Itoa(changedLines),
			fmt.Sprintf("%.2f", percentageChange),
			branch1,
			branch2,
		}
		fmt.Println(strings.Join(parts, separator))
	default:
		outputError("Invalid output format specified")
	}
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
