#!/usr/bin/env python3
"""
Aggregation script for MAPF-HET benchmark results.
Computes statistics and generates tables/graphs for paper.
"""

import argparse
import csv
import os
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional

import numpy as np

# Optional imports for visualization
try:
    import matplotlib.pyplot as plt
    HAS_MATPLOTLIB = True
except ImportError:
    HAS_MATPLOTLIB = False


@dataclass
class RunResult:
    """Single benchmark run result."""
    timestamp: str
    commit_hash: str
    instance: str
    num_agents: int
    num_tasks: int
    grid_size: str
    solver: str
    runtime_ms: float
    success: bool
    makespan: float
    deadline_met: bool
    num_conflicts: int
    nodes_expanded: int
    energy_violations: int
    deadline_violations: int


@dataclass
class SolverStats:
    """Aggregated statistics for a solver."""
    name: str
    runtimes: List[float] = field(default_factory=list)
    makespans: List[float] = field(default_factory=list)
    successes: int = 0
    total: int = 0
    deadlines_met: int = 0
    energy_violations: int = 0
    deadline_violations: int = 0
    nodes_expanded: List[int] = field(default_factory=list)

    def success_rate(self) -> float:
        return self.successes / self.total if self.total > 0 else 0.0

    def deadline_compliance(self) -> float:
        return self.deadlines_met / self.successes if self.successes > 0 else 0.0

    def mean_runtime(self) -> float:
        return np.mean(self.runtimes) if self.runtimes else 0.0

    def std_runtime(self) -> float:
        return np.std(self.runtimes) if self.runtimes else 0.0

    def percentile_runtime(self, p: float) -> float:
        return np.percentile(self.runtimes, p) if self.runtimes else 0.0

    def mean_makespan(self) -> float:
        return np.mean(self.makespans) if self.makespans else 0.0


def load_results(csv_path: str) -> List[RunResult]:
    """Load benchmark results from CSV file."""
    results = []
    with open(csv_path, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            results.append(RunResult(
                timestamp=row['timestamp'],
                commit_hash=row['commit_hash'],
                instance=row['instance'],
                num_agents=int(row['num_agents']),
                num_tasks=int(row['num_tasks']),
                grid_size=row['grid_size'],
                solver=row['solver'],
                runtime_ms=float(row['runtime_ms']),
                success=row['success'].lower() == 'true',
                makespan=float(row['makespan']),
                deadline_met=row['deadline_met'].lower() == 'true',
                num_conflicts=int(row['num_conflicts']),
                nodes_expanded=int(row['nodes_expanded']),
                energy_violations=int(row['energy_violations']),
                deadline_violations=int(row['deadline_violations']),
            ))
    return results


def aggregate_by_solver(results: List[RunResult]) -> Dict[str, SolverStats]:
    """Aggregate results by solver."""
    stats: Dict[str, SolverStats] = {}

    for r in results:
        if r.solver not in stats:
            stats[r.solver] = SolverStats(name=r.solver)
        s = stats[r.solver]
        s.total += 1
        if r.success:
            s.successes += 1
            s.runtimes.append(r.runtime_ms)
            s.makespans.append(r.makespan)
            s.nodes_expanded.append(r.nodes_expanded)
            if r.deadline_met:
                s.deadlines_met += 1
        s.energy_violations += r.energy_violations
        s.deadline_violations += r.deadline_violations

    return stats


def aggregate_by_solver_and_scale(results: List[RunResult]) -> Dict[str, Dict[int, SolverStats]]:
    """Aggregate results by solver and agent count (for scaling analysis)."""
    stats: Dict[str, Dict[int, SolverStats]] = defaultdict(dict)

    for r in results:
        if r.num_agents not in stats[r.solver]:
            stats[r.solver][r.num_agents] = SolverStats(name=r.solver)
        s = stats[r.solver][r.num_agents]
        s.total += 1
        if r.success:
            s.successes += 1
            s.runtimes.append(r.runtime_ms)
            s.makespans.append(r.makespan)
            s.nodes_expanded.append(r.nodes_expanded)
            if r.deadline_met:
                s.deadlines_met += 1
        s.energy_violations += r.energy_violations
        s.deadline_violations += r.deadline_violations

    return stats


def print_summary_table(stats: Dict[str, SolverStats]):
    """Print summary statistics table."""
    print("\n" + "=" * 100)
    print("BENCHMARK SUMMARY")
    print("=" * 100)

    header = f"{'Solver':<20} {'Runs':>6} {'Success%':>8} {'Mean(ms)':>10} {'Std(ms)':>10} {'P50(ms)':>10} {'P95(ms)':>10} {'P99(ms)':>10}"
    print(header)
    print("-" * 100)

    for name in sorted(stats.keys()):
        s = stats[name]
        print(f"{name:<20} {s.total:>6} {s.success_rate()*100:>7.1f}% "
              f"{s.mean_runtime():>10.2f} {s.std_runtime():>10.2f} "
              f"{s.percentile_runtime(50):>10.2f} {s.percentile_runtime(95):>10.2f} "
              f"{s.percentile_runtime(99):>10.2f}")


def print_deadline_table(stats: Dict[str, SolverStats]):
    """Print deadline compliance table."""
    print("\n" + "=" * 80)
    print("DEADLINE COMPLIANCE & ENERGY VIOLATIONS")
    print("=" * 80)

    header = f"{'Solver':<20} {'Deadline%':>12} {'EnergyViol':>12} {'DeadlineViol':>12}"
    print(header)
    print("-" * 80)

    for name in sorted(stats.keys()):
        s = stats[name]
        print(f"{name:<20} {s.deadline_compliance()*100:>11.1f}% "
              f"{s.energy_violations:>12} {s.deadline_violations:>12}")


def print_scaling_table(scale_stats: Dict[str, Dict[int, SolverStats]]):
    """Print scaling analysis table."""
    print("\n" + "=" * 120)
    print("SCALING ANALYSIS (Runtime in ms)")
    print("=" * 120)

    # Get all agent counts
    all_scales = set()
    for solver_stats in scale_stats.values():
        all_scales.update(solver_stats.keys())
    scales = sorted(all_scales)

    # Header
    header = f"{'Solver':<20}"
    for n in scales:
        header += f" {n:>12}"
    print(header)
    print("-" * 120)

    # Data rows
    for solver in sorted(scale_stats.keys()):
        row = f"{solver:<20}"
        for n in scales:
            if n in scale_stats[solver]:
                s = scale_stats[solver][n]
                row += f" {s.mean_runtime():>12.2f}"
            else:
                row += f" {'-':>12}"
        print(row)


def generate_latex_table(stats: Dict[str, SolverStats], output_path: str):
    """Generate LaTeX table for paper."""
    lines = [
        r"\begin{table}[htbp]",
        r"\centering",
        r"\caption{MAPF-HET Solver Performance Comparison}",
        r"\label{tab:solver-comparison}",
        r"\begin{tabular}{lrrrrrrr}",
        r"\toprule",
        r"Solver & Runs & Success\% & Mean (ms) & Std & P50 & P95 & P99 \\",
        r"\midrule",
    ]

    for name in sorted(stats.keys()):
        s = stats[name]
        lines.append(
            f"{name} & {s.total} & {s.success_rate()*100:.1f}\\% & "
            f"{s.mean_runtime():.2f} & {s.std_runtime():.2f} & "
            f"{s.percentile_runtime(50):.2f} & {s.percentile_runtime(95):.2f} & "
            f"{s.percentile_runtime(99):.2f} \\\\"
        )

    lines.extend([
        r"\bottomrule",
        r"\end{tabular}",
        r"\end{table}",
    ])

    with open(output_path, 'w') as f:
        f.write('\n'.join(lines))
    print(f"\nLaTeX table written to: {output_path}")


def generate_speedup_table(stats: Dict[str, SolverStats], baseline: str = "CBS-HET"):
    """Print speedup comparison table."""
    if baseline not in stats:
        print(f"Warning: Baseline solver '{baseline}' not found in results")
        return

    print("\n" + "=" * 60)
    print(f"SPEEDUP vs {baseline}")
    print("=" * 60)

    baseline_time = stats[baseline].mean_runtime()
    if baseline_time <= 0:
        print("Warning: Baseline has no successful runs")
        return

    header = f"{'Solver':<20} {'Mean(ms)':>12} {'Speedup':>12}"
    print(header)
    print("-" * 60)

    for name in sorted(stats.keys()):
        s = stats[name]
        speedup = baseline_time / s.mean_runtime() if s.mean_runtime() > 0 else 0
        print(f"{name:<20} {s.mean_runtime():>12.2f} {speedup:>11.2f}x")


def generate_plots(scale_stats: Dict[str, Dict[int, SolverStats]], output_dir: str):
    """Generate matplotlib plots for paper."""
    if not HAS_MATPLOTLIB:
        print("Warning: matplotlib not installed, skipping plots")
        return

    os.makedirs(output_dir, exist_ok=True)

    # Get all agent counts
    all_scales = set()
    for solver_stats in scale_stats.values():
        all_scales.update(solver_stats.keys())
    scales = sorted(all_scales)

    # Plot 1: Runtime scaling
    plt.figure(figsize=(10, 6))
    for solver in sorted(scale_stats.keys()):
        x = []
        y = []
        for n in scales:
            if n in scale_stats[solver]:
                x.append(n)
                y.append(scale_stats[solver][n].mean_runtime())
        if x:
            plt.plot(x, y, marker='o', label=solver)

    plt.xlabel('Number of Agents')
    plt.ylabel('Runtime (ms)')
    plt.title('MAPF-HET Solver Scaling')
    plt.legend()
    plt.xscale('log')
    plt.yscale('log')
    plt.grid(True, alpha=0.3)
    plt.savefig(os.path.join(output_dir, 'scaling_runtime.png'), dpi=300, bbox_inches='tight')
    plt.close()

    # Plot 2: Success rate by scale
    plt.figure(figsize=(10, 6))
    for solver in sorted(scale_stats.keys()):
        x = []
        y = []
        for n in scales:
            if n in scale_stats[solver]:
                x.append(n)
                y.append(scale_stats[solver][n].success_rate() * 100)
        if x:
            plt.plot(x, y, marker='o', label=solver)

    plt.xlabel('Number of Agents')
    plt.ylabel('Success Rate (%)')
    plt.title('MAPF-HET Solver Success Rate by Scale')
    plt.legend()
    plt.xscale('log')
    plt.ylim(0, 105)
    plt.grid(True, alpha=0.3)
    plt.savefig(os.path.join(output_dir, 'scaling_success.png'), dpi=300, bbox_inches='tight')
    plt.close()

    print(f"\nPlots written to: {output_dir}/")


def main():
    parser = argparse.ArgumentParser(description='Aggregate MAPF-HET benchmark results')
    parser.add_argument('input', help='Input CSV file with benchmark results')
    parser.add_argument('--output-dir', '-o', default='evidence',
                        help='Output directory for generated files')
    parser.add_argument('--latex', action='store_true',
                        help='Generate LaTeX table')
    parser.add_argument('--plots', action='store_true',
                        help='Generate matplotlib plots')
    parser.add_argument('--baseline', default='CBS-HET',
                        help='Baseline solver for speedup comparison')

    args = parser.parse_args()

    if not os.path.exists(args.input):
        print(f"Error: Input file not found: {args.input}")
        sys.exit(1)

    # Load results
    print(f"Loading results from: {args.input}")
    results = load_results(args.input)
    print(f"Loaded {len(results)} benchmark results")

    # Aggregate
    stats = aggregate_by_solver(results)
    scale_stats = aggregate_by_solver_and_scale(results)

    # Print tables
    print_summary_table(stats)
    print_deadline_table(stats)
    print_scaling_table(scale_stats)
    generate_speedup_table(stats, args.baseline)

    # Generate output files
    os.makedirs(args.output_dir, exist_ok=True)

    if args.latex:
        generate_latex_table(stats, os.path.join(args.output_dir, 'solver_table.tex'))

    if args.plots:
        generate_plots(scale_stats, args.output_dir)


if __name__ == '__main__':
    main()
