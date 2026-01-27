// Package main provides benchmark runner for MAPF-HET solvers.
// Runs all solvers on test instances and collects metrics.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// InstanceFile represents a loaded instance.
type InstanceFile struct {
	Name   string `json:"name"`
	Params struct {
		Seed       int64 `json:"seed"`
		NumAgents  int   `json:"num_agents"`
		GridWidth  int   `json:"grid_width"`
		GridHeight int   `json:"grid_height"`
		TaskCount  int   `json:"task_count"`
	} `json:"params"`
	Vertices []interface{} `json:"vertices"`
	Edges    []interface{} `json:"edges"`
	Robots   []interface{} `json:"robots"`
	Tasks    []interface{} `json:"tasks"`
	Deadline float64       `json:"deadline"`
}

// BenchmarkResult stores results from a single solver run.
type BenchmarkResult struct {
	Timestamp      string  `json:"timestamp"`
	CommitHash     string  `json:"commit_hash"`
	GoVersion      string  `json:"go_version"`
	OS             string  `json:"os"`
	Arch           string  `json:"arch"`
	Instance       string  `json:"instance"`
	NumAgents      int     `json:"num_agents"`
	NumTasks       int     `json:"num_tasks"`
	GridSize       string  `json:"grid_size"`
	Solver         string  `json:"solver"`
	RuntimeMs      float64 `json:"runtime_ms"`
	Success        bool    `json:"success"`
	Makespan       float64 `json:"makespan"`
	DeadlineMet    bool    `json:"deadline_met"`
	NumConflicts   int     `json:"num_conflicts"`
	NodesExpanded  int     `json:"nodes_expanded"`
	EnergyViolations int   `json:"energy_violations"`
	DeadlineViolations int `json:"deadline_violations"`
}

// SolverMetrics holds per-solver aggregated metrics.
type SolverMetrics struct {
	Name           string
	TotalRuns      int
	Successes      int
	TotalRuntimeMs float64
	TotalMakespan  float64
	DeadlinesMet   int
	EnergyViolations int
}

var solvers = []string{
	"Prioritized",
	"CBS-HET",
	"HybridCBS-HET",
	"MIXED-CBS-HET",
	"DeadlineCBS-HET",
	"StochasticECBS-HET",
}

func getGitCommit() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func loadInstance(path string) (*InstanceFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var inst InstanceFile
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, err
	}

	return &inst, nil
}

// runSolver executes the mapfhet CLI and parses results.
// This is a placeholder - actual implementation would invoke the solver directly.
func runSolver(inst *InstanceFile, solverName string, timeout time.Duration) *BenchmarkResult {
	result := &BenchmarkResult{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		CommitHash: getGitCommit(),
		GoVersion:  runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		Instance:   inst.Name,
		NumAgents:  inst.Params.NumAgents,
		NumTasks:   inst.Params.TaskCount,
		GridSize:   fmt.Sprintf("%dx%d", inst.Params.GridWidth, inst.Params.GridHeight),
		Solver:     solverName,
	}

	// Note: This is a stub. The actual implementation should:
	// 1. Convert the JSON instance to core.Instance
	// 2. Create the appropriate solver
	// 3. Run the solver and measure time
	// 4. Collect metrics from the solution
	//
	// For now, we'll call the mapfhet binary with appropriate flags
	// once it supports benchmark mode.

	startTime := time.Now()

	// Placeholder: simulate solver execution
	// In real implementation, this would call:
	// solver := algo.NewMixedCBS(inst.Deadline)
	// solution := solver.Solve(coreInstance)
	result.RuntimeMs = float64(time.Since(startTime).Microseconds()) / 1000.0
	result.Success = false // Will be set by actual solver

	return result
}

func writeCSV(results []*BenchmarkResult, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header
	header := []string{
		"timestamp", "commit_hash", "go_version", "os", "arch",
		"instance", "num_agents", "num_tasks", "grid_size", "solver",
		"runtime_ms", "success", "makespan", "deadline_met",
		"num_conflicts", "nodes_expanded", "energy_violations", "deadline_violations",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Data rows
	for _, r := range results {
		row := []string{
			r.Timestamp, r.CommitHash, r.GoVersion, r.OS, r.Arch,
			r.Instance, fmt.Sprintf("%d", r.NumAgents), fmt.Sprintf("%d", r.NumTasks),
			r.GridSize, r.Solver,
			fmt.Sprintf("%.3f", r.RuntimeMs), fmt.Sprintf("%t", r.Success),
			fmt.Sprintf("%.3f", r.Makespan), fmt.Sprintf("%t", r.DeadlineMet),
			fmt.Sprintf("%d", r.NumConflicts), fmt.Sprintf("%d", r.NodesExpanded),
			fmt.Sprintf("%d", r.EnergyViolations), fmt.Sprintf("%d", r.DeadlineViolations),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func printSummary(results []*BenchmarkResult) {
	// Aggregate by solver
	metrics := make(map[string]*SolverMetrics)
	for _, r := range results {
		m, ok := metrics[r.Solver]
		if !ok {
			m = &SolverMetrics{Name: r.Solver}
			metrics[r.Solver] = m
		}
		m.TotalRuns++
		if r.Success {
			m.Successes++
			m.TotalRuntimeMs += r.RuntimeMs
			m.TotalMakespan += r.Makespan
			if r.DeadlineMet {
				m.DeadlinesMet++
			}
		}
		m.EnergyViolations += r.EnergyViolations
	}

	// Print summary table
	fmt.Println("\n=== BENCHMARK SUMMARY ===")
	fmt.Printf("%-20s %8s %8s %12s %10s %8s %10s\n",
		"Solver", "Runs", "Success", "Avg Time(ms)", "AvgMakespan", "Deadline%", "EnergyViol")
	fmt.Println(strings.Repeat("-", 78))

	var names []string
	for name := range metrics {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		m := metrics[name]
		avgTime := 0.0
		avgMakespan := 0.0
		deadlinePct := 0.0
		if m.Successes > 0 {
			avgTime = m.TotalRuntimeMs / float64(m.Successes)
			avgMakespan = m.TotalMakespan / float64(m.Successes)
			deadlinePct = float64(m.DeadlinesMet) / float64(m.Successes) * 100
		}
		fmt.Printf("%-20s %8d %8d %12.2f %10.2f %7.1f%% %10d\n",
			m.Name, m.TotalRuns, m.Successes, avgTime, avgMakespan, deadlinePct, m.EnergyViolations)
	}
}

func main() {
	inputDir := flag.String("input", "testdata", "Directory containing instance JSON files")
	outputFile := flag.String("output", "evidence/benchmark_results.csv", "Output CSV file")
	timeout := flag.Duration("timeout", 5*time.Minute, "Timeout per solver run")
	solverFilter := flag.String("solver", "", "Run only specific solver (comma-separated)")
	agentFilter := flag.Int("agents", 0, "Run only instances with this many agents (0 = all)")
	verbose := flag.Bool("verbose", false, "Verbose output")

	flag.Parse()

	// Create output directory
	outputDir := filepath.Dir(*outputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Find instance files
	pattern := filepath.Join(*inputDir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding instance files: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No instance files found in %s\n", *inputDir)
		fmt.Fprintf(os.Stderr, "Run gen_instances first: go run gen_instances.go -scaling -output testdata\n")
		os.Exit(1)
	}

	// Parse solver filter
	activeSolvers := solvers
	if *solverFilter != "" {
		activeSolvers = strings.Split(*solverFilter, ",")
	}

	var results []*BenchmarkResult
	totalRuns := len(files) * len(activeSolvers)
	currentRun := 0

	fmt.Printf("Running benchmarks: %d instances x %d solvers = %d runs\n",
		len(files), len(activeSolvers), totalRuns)
	fmt.Printf("Timeout per run: %v\n", *timeout)
	fmt.Println()

	for _, file := range files {
		inst, err := loadInstance(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", file, err)
			continue
		}

		// Filter by agent count
		if *agentFilter > 0 && inst.Params.NumAgents != *agentFilter {
			continue
		}

		for _, solver := range activeSolvers {
			currentRun++
			if *verbose {
				fmt.Printf("[%d/%d] %s / %s ... ", currentRun, totalRuns, inst.Name, solver)
			} else {
				fmt.Printf("\r[%d/%d] Running...", currentRun, totalRuns)
			}

			result := runSolver(inst, solver, *timeout)
			results = append(results, result)

			if *verbose {
				if result.Success {
					fmt.Printf("OK (%.2fms, makespan=%.2f)\n", result.RuntimeMs, result.Makespan)
				} else {
					fmt.Printf("FAILED\n")
				}
			}
		}
	}

	fmt.Println()

	// Write results
	if err := writeCSV(results, *outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing results: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Results written to: %s\n", *outputFile)

	// Print summary
	printSummary(results)
}
