#!/usr/bin/env python3
"""
Claim verification script for MAPF-HET paper.
Verifies that benchmark results support the claims made in the paper.
"""

import argparse
import csv
import json
import os
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Tuple

import numpy as np


# Paper claims to verify
CLAIMS = {
    "speedup_mixed_vs_cbs": {
        "description": "5.6x speedup of MIXED-CBS over standard CBS",
        "target": 5.6,
        "tolerance": 0.5,  # Allow 5.1-6.1x
        "section": "Section V-A",
    },
    "deadline_compliance_24h": {
        "description": "99.9% deadline compliance over 24h simulation",
        "target": 99.9,
        "tolerance": 0.5,  # Allow 99.4%+
        "section": "Section V-F",
    },
    "zero_energy_violations": {
        "description": "Zero energy violations over 24h simulation",
        "target": 0,
        "tolerance": 0,  # Must be exactly 0
        "section": "Section V-E",
    },
    "scaling_ologn": {
        "description": "O(log N) convergence at 2048 modules",
        "target": "O(log N)",
        "section": "Section IV-C",
    },
    "p99_latency_1ms": {
        "description": "P99 latency < 1ms at 2048 modules",
        "target": 1.0,  # ms
        "section": "Section V-D",
    },
    "canfd_bandwidth_42pct": {
        "description": "CAN-FD bandwidth utilization 42%",
        "target": 42.0,
        "tolerance": 5.0,  # Allow 37-47%
        "section": "Section V-G",
    },
}


@dataclass
class ClaimResult:
    """Result of verifying a single claim."""
    claim_id: str
    description: str
    section: str
    target: str
    actual: str
    passed: bool
    details: str = ""


@dataclass
class VerificationReport:
    """Complete verification report."""
    timestamp: str
    commit_hash: str
    total_runs: int
    claims: List[ClaimResult] = field(default_factory=list)

    def passed_count(self) -> int:
        return sum(1 for c in self.claims if c.passed)

    def failed_count(self) -> int:
        return sum(1 for c in self.claims if not c.passed)

    def overall_pass(self) -> bool:
        return all(c.passed for c in self.claims)


def load_results(csv_path: str) -> List[Dict]:
    """Load benchmark results from CSV."""
    results = []
    with open(csv_path, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            row['runtime_ms'] = float(row['runtime_ms'])
            row['makespan'] = float(row['makespan'])
            row['num_agents'] = int(row['num_agents'])
            row['success'] = row['success'].lower() == 'true'
            row['deadline_met'] = row['deadline_met'].lower() == 'true'
            row['energy_violations'] = int(row['energy_violations'])
            row['deadline_violations'] = int(row['deadline_violations'])
            results.append(row)
    return results


def compute_speedup(results: List[Dict], solver1: str, solver2: str) -> Tuple[float, str]:
    """Compute speedup of solver1 over solver2."""
    times1 = [r['runtime_ms'] for r in results if r['solver'] == solver1 and r['success']]
    times2 = [r['runtime_ms'] for r in results if r['solver'] == solver2 and r['success']]

    if not times1 or not times2:
        return 0.0, f"Insufficient data: {len(times1)} runs for {solver1}, {len(times2)} runs for {solver2}"

    mean1 = np.mean(times1)
    mean2 = np.mean(times2)

    if mean1 <= 0:
        return 0.0, f"Invalid mean runtime for {solver1}: {mean1}"

    speedup = mean2 / mean1
    details = (f"{solver1}: {mean1:.2f}ms (n={len(times1)}), "
               f"{solver2}: {mean2:.2f}ms (n={len(times2)})")

    return speedup, details


def compute_deadline_compliance(results: List[Dict]) -> Tuple[float, str]:
    """Compute deadline compliance rate."""
    successful = [r for r in results if r['success']]
    if not successful:
        return 0.0, "No successful runs"

    met = sum(1 for r in successful if r['deadline_met'])
    rate = (met / len(successful)) * 100

    details = f"{met}/{len(successful)} deadlines met"
    return rate, details


def compute_energy_violations(results: List[Dict]) -> Tuple[int, str]:
    """Count total energy violations."""
    total = sum(r['energy_violations'] for r in results)
    details = f"Total violations across {len(results)} runs"
    return total, details


def compute_scaling_complexity(results: List[Dict], solver: str) -> Tuple[str, str]:
    """Estimate scaling complexity from runtime vs agent count."""
    # Group by agent count
    by_scale = {}
    for r in results:
        if r['solver'] == solver and r['success']:
            n = r['num_agents']
            if n not in by_scale:
                by_scale[n] = []
            by_scale[n].append(r['runtime_ms'])

    if len(by_scale) < 3:
        return "INSUFFICIENT DATA", f"Need at least 3 different scales, got {len(by_scale)}"

    # Compute mean runtime at each scale
    scales = sorted(by_scale.keys())
    runtimes = [np.mean(by_scale[n]) for n in scales]

    # Fit log(n) model
    log_scales = np.log2(scales)
    coeffs = np.polyfit(log_scales, runtimes, 1)
    predicted = np.polyval(coeffs, log_scales)
    r2_log = 1 - np.sum((runtimes - predicted)**2) / np.sum((runtimes - np.mean(runtimes))**2)

    # Fit linear model
    coeffs_lin = np.polyfit(scales, runtimes, 1)
    predicted_lin = np.polyval(coeffs_lin, scales)
    r2_lin = 1 - np.sum((runtimes - predicted_lin)**2) / np.sum((runtimes - np.mean(runtimes))**2)

    # Fit n^2 model
    sq_scales = [n**2 for n in scales]
    coeffs_sq = np.polyfit(sq_scales, runtimes, 1)
    predicted_sq = np.polyval(coeffs_sq, sq_scales)
    r2_sq = 1 - np.sum((runtimes - predicted_sq)**2) / np.sum((runtimes - np.mean(runtimes))**2)

    details = f"R² fits: O(log N)={r2_log:.3f}, O(N)={r2_lin:.3f}, O(N²)={r2_sq:.3f}"

    # Determine best fit
    if r2_log > r2_lin and r2_log > r2_sq:
        return "O(log N)", details
    elif r2_lin > r2_sq:
        return "O(N)", details
    else:
        return "O(N²)", details


def compute_p99_latency(results: List[Dict], solver: str, min_agents: int = 2048) -> Tuple[float, str]:
    """Compute P99 latency for large-scale instances."""
    filtered = [r for r in results
                if r['solver'] == solver and r['success'] and r['num_agents'] >= min_agents]

    if not filtered:
        return -1.0, f"No successful runs with {min_agents}+ agents for {solver}"

    runtimes = [r['runtime_ms'] for r in filtered]
    p99 = np.percentile(runtimes, 99)

    details = f"P99 from {len(runtimes)} runs with {min_agents}+ agents"
    return p99, details


def verify_claim_speedup(results: List[Dict]) -> ClaimResult:
    """Verify speedup claim."""
    claim = CLAIMS["speedup_mixed_vs_cbs"]
    speedup, details = compute_speedup(results, "MIXED-CBS-HET", "CBS-HET")

    passed = abs(speedup - claim["target"]) <= claim["tolerance"]

    return ClaimResult(
        claim_id="speedup_mixed_vs_cbs",
        description=claim["description"],
        section=claim["section"],
        target=f"{claim['target']}x",
        actual=f"{speedup:.2f}x",
        passed=passed,
        details=details,
    )


def verify_claim_deadline(results: List[Dict]) -> ClaimResult:
    """Verify deadline compliance claim."""
    claim = CLAIMS["deadline_compliance_24h"]
    rate, details = compute_deadline_compliance(results)

    passed = rate >= claim["target"] - claim["tolerance"]

    return ClaimResult(
        claim_id="deadline_compliance_24h",
        description=claim["description"],
        section=claim["section"],
        target=f"{claim['target']}%",
        actual=f"{rate:.2f}%",
        passed=passed,
        details=details,
    )


def verify_claim_energy(results: List[Dict]) -> ClaimResult:
    """Verify zero energy violations claim."""
    claim = CLAIMS["zero_energy_violations"]
    violations, details = compute_energy_violations(results)

    passed = violations <= claim["target"]

    return ClaimResult(
        claim_id="zero_energy_violations",
        description=claim["description"],
        section=claim["section"],
        target=str(claim["target"]),
        actual=str(violations),
        passed=passed,
        details=details,
    )


def verify_claim_scaling(results: List[Dict]) -> ClaimResult:
    """Verify O(log N) scaling claim."""
    claim = CLAIMS["scaling_ologn"]
    complexity, details = compute_scaling_complexity(results, "MIXED-CBS-HET")

    passed = complexity == "O(log N)"

    return ClaimResult(
        claim_id="scaling_ologn",
        description=claim["description"],
        section=claim["section"],
        target=claim["target"],
        actual=complexity,
        passed=passed,
        details=details,
    )


def verify_claim_p99(results: List[Dict]) -> ClaimResult:
    """Verify P99 latency claim."""
    claim = CLAIMS["p99_latency_1ms"]
    p99, details = compute_p99_latency(results, "MIXED-CBS-HET", min_agents=2048)

    if p99 < 0:
        passed = False
    else:
        passed = p99 < claim["target"]

    actual = f"{p99:.3f}ms" if p99 >= 0 else "N/A"

    return ClaimResult(
        claim_id="p99_latency_1ms",
        description=claim["description"],
        section=claim["section"],
        target=f"<{claim['target']}ms",
        actual=actual,
        passed=passed,
        details=details,
    )


def run_verification(results: List[Dict]) -> VerificationReport:
    """Run all claim verifications."""
    report = VerificationReport(
        timestamp=results[0]['timestamp'] if results else "N/A",
        commit_hash=results[0]['commit_hash'] if results else "N/A",
        total_runs=len(results),
    )

    # Run each verification
    report.claims.append(verify_claim_speedup(results))
    report.claims.append(verify_claim_deadline(results))
    report.claims.append(verify_claim_energy(results))
    report.claims.append(verify_claim_scaling(results))
    report.claims.append(verify_claim_p99(results))

    return report


def print_report(report: VerificationReport):
    """Print verification report."""
    print("\n" + "=" * 100)
    print("PAPER CLAIM VERIFICATION REPORT")
    print("=" * 100)
    print(f"Timestamp: {report.timestamp}")
    print(f"Commit: {report.commit_hash}")
    print(f"Total benchmark runs: {report.total_runs}")
    print()

    for claim in report.claims:
        status = "PASS" if claim.passed else "FAIL"
        status_color = "\033[92m" if claim.passed else "\033[91m"  # Green/Red
        reset = "\033[0m"

        print(f"[{status_color}{status}{reset}] {claim.description}")
        print(f"       Section: {claim.section}")
        print(f"       Target:  {claim.target}")
        print(f"       Actual:  {claim.actual}")
        if claim.details:
            print(f"       Details: {claim.details}")
        print()

    print("-" * 100)
    overall = "ALL CLAIMS VERIFIED" if report.overall_pass() else "SOME CLAIMS FAILED"
    print(f"OVERALL: {report.passed_count()}/{len(report.claims)} claims passed - {overall}")
    print("=" * 100)


def save_report_json(report: VerificationReport, output_path: str):
    """Save report as JSON."""
    data = {
        "timestamp": report.timestamp,
        "commit_hash": report.commit_hash,
        "total_runs": report.total_runs,
        "overall_pass": report.overall_pass(),
        "passed_count": report.passed_count(),
        "failed_count": report.failed_count(),
        "claims": [
            {
                "id": c.claim_id,
                "description": c.description,
                "section": c.section,
                "target": c.target,
                "actual": c.actual,
                "passed": c.passed,
                "details": c.details,
            }
            for c in report.claims
        ]
    }

    with open(output_path, 'w') as f:
        json.dump(data, f, indent=2)

    print(f"\nReport saved to: {output_path}")


def main():
    parser = argparse.ArgumentParser(description='Verify MAPF-HET paper claims against benchmark results')
    parser.add_argument('input', help='Input CSV file with benchmark results')
    parser.add_argument('--output', '-o', help='Output JSON report file')
    parser.add_argument('--strict', action='store_true',
                        help='Exit with error code if any claim fails')

    args = parser.parse_args()

    if not os.path.exists(args.input):
        print(f"Error: Input file not found: {args.input}")
        sys.exit(1)

    # Load results
    print(f"Loading results from: {args.input}")
    results = load_results(args.input)
    print(f"Loaded {len(results)} benchmark results")

    # Run verification
    report = run_verification(results)

    # Print report
    print_report(report)

    # Save JSON report
    if args.output:
        save_report_json(report, args.output)

    # Exit code
    if args.strict and not report.overall_pass():
        sys.exit(1)


if __name__ == '__main__':
    main()
