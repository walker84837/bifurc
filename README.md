# diffstat

> Compare how different two Git branches are.

[![Build Source Code](https://github.com/walker84837/diffstat/actions/workflows/build.yml/badge.svg)](https://github.com/walker84837/diffstat/actions/workflows/build.yml)
[![License: MPL-2.0](https://img.shields.io/badge/License-MPL--2.0-blue.svg)](LICENSE)

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Flags](#flags)
- [How It Works](#how-it-works)
  - [Formula](#formula)
  - [Presets by Codebase Type](#presets-by-codebase-type)
  - [Auto-calculated Lambda](#auto-calculated-lambda)
  - [Sensitivity](#sensitivity)
- [Output Formats](#output-formats)
  - [Text (default)](#text-default)
  - [JSON](#json)
  - [Custom](#custom)
- [Interpreting the Divergence Percentage](#interpreting-the-divergence-percentage)
- [Contributing](#contributing)
- [License](#license)

## Installation

```bash
git clone https://github.com/walker84837/diffstat.git
cd diffstat
go build
```

## Usage

```bash
./diffstat --branch1 main --branch2 feature-branch
```

This compares the two branches and outputs a divergence score from 0% to near 100%.

## Flags

| Flag | Default | Description |
|---|:---:|---|
| `--branch1` | `""` | First branch (required) |
| `--branch2` | `""` | Second branch (required) |
| `--format` | `text` | Output format: `text`, `json`, `custom` |
| `--separator` | `\n` | Field separator for custom format |
| `--no-color` | `false` | Disable colored output |
| `--preset` | `custom` | Codebase type preset: `systems`, `web`, `ml`, `cli`, `library`, `custom` |
| `--sensitivity` | `normal` | Curve sensitivity: `low`, `normal`, `high` |
| `--lambda` | `0` | Manual lambda override (0 = auto-calculate) |
| `--text-weight` | `0.85` | Text change weight (only with `--preset custom`) |
| `--binary-weight` | `0.15` | Binary change weight (only with `--preset custom`) |

### Examples

```bash
# Basic comparison
./diffstat --branch1 main --branch2 feature-branch

# Machine-readable JSON output
./diffstat --branch1 main --branch2 feature-branch --format json

# ML project preset (higher binary weight for model weights)
./diffstat --branch1 main --branch2 feature-branch --preset ml

# Gentle curve, scripting-friendly output
./diffstat --branch1 develop --branch2 main --sensitivity low --format custom --separator $'\t'

# Manual lambda override (λ = 1.5)
./diffstat --branch1 HEAD~5 --branch2 HEAD --lambda 1.5

# Custom weights for a unique codebase
./diffstat --branch1 main --branch2 experiment --preset custom --text-weight 0.9 --binary-weight 0.1

# Disable colors for CI pipelines
./diffstat --branch1 main --branch2 feature --no-color
```

## How It Works

diffstat calculates a **divergence score** between two Git branches using a formula that accounts for both text and binary file changes, normalized to the size of the codebase.

### Formula

$$
D = \frac{\Delta L}{\max(LOC_1, LOC_2)} \times W_t + \frac{\Delta B}{\max(Bytes_1, Bytes_2)} \times W_b
$$

$$
\text{Divergence}\% = 100 \times \left(1 - e^{-\lambda D}\right)
$$

| Symbol | Meaning |
|---|---|
| ΔL | Total lines added + deleted across all text files (from `git diff --numstat`) |
| LOC₁, LOC₂ | Total lines of code in each branch |
| ΔBytes | Sum of \|size₁ − size₂\| for each changed binary file |
| Bytes₁, Bytes₂ | Total byte size of all binary files in each branch |
| Wt, Wb | Text and binary weights (from preset or `--text-weight` / `--binary-weight`) |
| λ | Lambda — controls how steep the exponential curve is |
| e | Euler's number |

The formula uses `max()` for denominators so the result is **symmetric** — swapping branch order gives the same divergence score.

### Presets by Codebase Type

Presets set Wt and Wb to sensible defaults for different project types:

| Preset | Wt | Wb | Best for |
|---|---|---|---|
| `systems` | 0.95 | 0.05 | C, C++, Rust, embedded — binaries are compiled output or firmware blobs |
| `web` | 0.75 | 0.25 | Websites, frontend apps — images, fonts, and icons carry real weight |
| `ml` | 0.60 | 0.40 | AI/ML — model weights, datasets, and checkpoints are core artifacts |
| `cli` | 0.92 | 0.08 | CLI tools — almost purely logic-driven |
| `library` | 0.88 | 0.12 | General-purpose libraries — binaries are test fixtures or native extensions |
| `custom` | 0.85 | 0.15 | Your own weights via `--text-weight` and `--binary-weight` |

### Auto-calculated Lambda

When `--lambda` is 0 (default), λ is auto-calculated from the larger branch's line count:

$$
\lambda = \text{clamp}\!\left(0.5,\ \frac{4.0}{\log_{10}(\max(LOC, 10))},\ 2.0\right)
$$

| LOC | λ |
|---|---|
| < 100 | 2.00 |
| 1,000 | 1.33 |
| 10,000 | 1.00 |
| 100,000 | 0.80 |
| 1,000,000 | 0.67 |

This ensures small repos are more sensitive (small changes feel significant) and large repos are less sensitive (small changes don't look alarming).

### Sensitivity

The `--sensitivity` flag scales the auto-calculated λ:

| Sensitivity | Multiplier | Effect |
|---|---|---|
| `low` | × 0.6 | Gentler curve — harder to reach high divergence |
| `normal` | × 1.0 | Default — the auto-calculated value |
| `high` | × 1.5 | Steeper curve — divergence feels more urgent |

Use `--lambda N` to override entirely, bypassing both auto-calculation and sensitivity.

## Output Formats

### Text (default)

```
Repository: my-repo (web preset) (λ=1.00, sensitivity=normal)

  Total LOC:          10,042
  Total binary:        2.3 MB
  Lines changed:         142  (1.4% of LOC)
  Binary changed:      1.2 KB  (0.1% of binaries)
  Raw score D:         0.023
  Divergence:           3.1%
```

### JSON

```json
{
  "totalLoc": 10042,
  "totalBinaryBytes": 2359296,
  "deltaLines": 142,
  "deltaBinaryBytes": 1228,
  "rawScore": 0.023,
  "divergencePercent": 3.10,
  "lambda": 1.00,
  "sensitivity": "normal",
  "preset": "web",
  "textWeight": 0.75,
  "binaryWeight": 0.25,
  "branch1": "main",
  "branch2": "feature"
}
```

### Custom

Tab-separated fields for scripting (customize separator with `--separator`):

```
divergencePercent  rawScore  deltaLines  deltaBinaryBytes  totalLoc  totalBinaryBytes  lambda  wt  wb  preset  sensitivity  branch1  branch2
```

Example:
```bash
./diffstat --branch1 main --branch2 feature --format custom --separator $'\t'
```

## Interpreting the Divergence Percentage

The divergence percentage follows an **exponential curve** — small changes produce proportional results, but large changes compress as they approach 100%. This makes the scale feel natural across repos of any size.

| Divergence | Interpretation |
|---|---|
| **0%** | Branches are identical (no diff) |
| **~1–5%** | Routine change — small hotfix or minor PR |
| **~5–20%** | Normal feature branch — measured work |
| **~20–40%** | Large PR or short-lived fork |
| **~40–65%** | Significant divergence — weeks of parallel work |
| **~65–85%** | Heavy divergence — near-rewrite of substantial parts |
| **85–100%** | Extreme divergence — completely independent branches |

The curve is asymptotic: you will never reach exactly 100% (unless D is infinite). This is intentional — even a total rewrite still shares some structure (directory layout, config files, etc.) with the original.

## Contributing

Contributions are welcome. Open an issue or submit a pull request on [GitHub](https://github.com/walker84837/diffstat).

## License

MPL-2.0. See [LICENSE](LICENSE).
