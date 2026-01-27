# MAPF-HET Benchmark Tools

Tools for generating benchmark instances, running solvers, and verifying paper claims.

## Quick Start

```bash
# 1. Generate test instances (scaling suite: 10-2048 agents)
cd tools/gen_instances
go run main.go -scaling -output ../../testdata

# 2. Run benchmarks
cd ../run_benchmarks
go run main.go -input ../../testdata -output ../../evidence/mapf-het/results.csv

# 3. Aggregate results and generate tables
cd ..
python summarize.py ../evidence/mapf-het/results.csv -o ../evidence/mapf-het --latex --plots

# 4. Verify paper claims
python verify_claims.py ../evidence/mapf-het/results.csv -o ../evidence/mapf-het/verification.json
```

## Tools

### gen_instances (Go)

Generates deterministic MAPF-HET problem instances.

```bash
go run main.go [flags]

Flags:
  -seed int          Random seed (default 42)
  -agents int        Number of agents (default 10)
  -width int         Grid width (default 10)
  -height int        Grid height (default 10)
  -tasks int         Number of tasks (default 20)
  -deadline-min      Minimum task deadline in seconds (default 300)
  -deadline-max      Maximum task deadline in seconds (default 600)
  -charging float    Charging pad density 0-1 (default 0.1)
  -drones float      Fraction of drone agents (default 0.2)
  -rail float        Fraction of rail agents (default 0.1)
  -layers int        Number of airspace layers (default 2)
  -output string     Output directory (default "testdata")
  -scaling           Generate scaling suite (10, 50, 100, 500, 1000, 2048 agents)
```

### run_benchmarks (Go)

Runs all MAPF-HET solvers on test instances.

```bash
go run main.go [flags]

Flags:
  -input string      Instance directory (default "testdata")
  -output string     Results CSV file (default "evidence/benchmark_results.csv")
  -timeout duration  Timeout per run (default 5m)
  -solver string     Run specific solvers (comma-separated)
  -agents int        Run only instances with N agents (0 = all)
  -verbose           Verbose output
```

### summarize.py (Python)

Aggregates benchmark results and generates statistics.

```bash
python summarize.py <input.csv> [options]

Options:
  -o, --output-dir   Output directory for generated files
  --latex            Generate LaTeX table
  --plots            Generate matplotlib plots
  --baseline         Baseline solver for speedup (default: CBS-HET)
```

**Output:**
- Summary statistics (mean, std, P50/P95/P99)
- Deadline compliance and energy violation tables
- Scaling analysis table
- Speedup comparison table
- LaTeX table for paper (`solver_table.tex`)
- Plots: `scaling_runtime.png`, `scaling_success.png`

### verify_claims.py (Python)

Verifies paper claims against benchmark results.

```bash
python verify_claims.py <input.csv> [options]

Options:
  -o, --output       Output JSON report
  --strict           Exit with error if any claim fails
```

**Verified Claims:**
1. 5.6x speedup of MIXED-CBS over CBS (Section V-A)
2. 99.9% deadline compliance (Section V-F)
3. Zero energy violations (Section V-E)
4. O(log N) convergence at 2048 modules (Section IV-C)
5. P99 latency < 1ms at 2048 modules (Section V-D)

## Instance JSON Format

```json
{
  "name": "mapfhet_50_15x15_42",
  "params": {
    "seed": 42,
    "num_agents": 50,
    "grid_width": 15,
    "grid_height": 15,
    "task_count": 100,
    "deadline_min": 300,
    "deadline_max": 600,
    "charging_density": 0.1,
    "drone_ratio": 0.2,
    "rail_ratio": 0.1,
    "airspace_layers": 2
  },
  "vertices": [...],
  "edges": [...],
  "robots": [...],
  "tasks": [...],
  "deadline": 720
}
```

## Evidence Structure

```
evidence/
└── mapf-het/
    ├── results.csv           # Raw benchmark results
    ├── verification.json     # Claim verification report
    ├── solver_table.tex      # LaTeX table for paper
    ├── scaling_runtime.png   # Runtime scaling plot
    └── scaling_success.png   # Success rate plot
```

## Dependencies

**Go tools:**
- Go 1.21+

**Python tools:**
- Python 3.8+
- numpy
- matplotlib (optional, for plots)

Install Python dependencies:
```bash
pip install numpy matplotlib
```
