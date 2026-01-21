// Package algo implements MAPF-HET solving algorithms.
package algo

import (
	"math"
	"sort"

	"github.com/elektrokombinacija/mapf-het-research/internal/core"
)

// Solver is the interface for MAPF-HET algorithms.
type Solver interface {
	// Solve attempts to find a solution for the instance.
	// Returns nil solution if no solution found.
	Solve(inst *core.Instance) *core.Solution

	// Name returns the algorithm name.
	Name() string
}

// Conflict represents a collision between two robots.
type Conflict struct {
	Robot1, Robot2 core.RobotID
	Vertex         core.VertexID
	Time           float64
	IsEdge         bool // Edge conflict vs vertex conflict
	// For edge conflicts: the two vertices being swapped
	EdgeFrom, EdgeTo core.VertexID
}

// Constraint prohibits a robot from being at a vertex at a time.
type Constraint struct {
	Robot  core.RobotID
	Vertex core.VertexID
	Time   float64
	// For edge constraints
	IsEdge   bool
	EdgeFrom core.VertexID
	EdgeTo   core.VertexID
}

// TimeTolerance for floating-point time comparison.
const TimeTolerance = 0.001

// timeEqual compares times with tolerance.
func timeEqual(t1, t2 float64) bool {
	return math.Abs(t1-t2) < TimeTolerance
}

// sortedRobotIDs returns sorted robot IDs from paths map.
func sortedRobotIDs(paths map[core.RobotID]core.Path) []core.RobotID {
	robots := make([]core.RobotID, 0, len(paths))
	for rid := range paths {
		robots = append(robots, rid)
	}
	sort.Slice(robots, func(i, j int) bool {
		return robots[i] < robots[j]
	})
	return robots
}

// getPositionAtTime returns robot position at given time via interpolation.
func getPositionAtTime(path core.Path, t float64) (core.VertexID, bool) {
	if len(path) == 0 {
		return 0, false
	}

	// Before path start
	if t < path[0].T-TimeTolerance {
		return path[0].V, true
	}

	// After path end - stay at last position
	if t > path[len(path)-1].T+TimeTolerance {
		return path[len(path)-1].V, true
	}

	// Find position at time t
	for i := 0; i < len(path); i++ {
		if timeEqual(path[i].T, t) {
			return path[i].V, true
		}
		if path[i].T > t && i > 0 {
			// Between path[i-1] and path[i] - return previous vertex
			return path[i-1].V, true
		}
	}

	return path[len(path)-1].V, true
}

// FindFirstConflict detects the first conflict in paths.
func FindFirstConflict(paths map[core.RobotID]core.Path) *Conflict {
	robots := sortedRobotIDs(paths)

	// Collect all unique time points
	timeSet := make(map[float64]bool)
	for _, path := range paths {
		for _, tv := range path {
			timeSet[tv.T] = true
		}
	}

	times := make([]float64, 0, len(timeSet))
	for t := range timeSet {
		times = append(times, t)
	}
	sort.Float64s(times)

	// Check for vertex conflicts at each time point
	for _, t := range times {
		for i := 0; i < len(robots); i++ {
			for j := i + 1; j < len(robots); j++ {
				path1, path2 := paths[robots[i]], paths[robots[j]]

				pos1, ok1 := getPositionAtTime(path1, t)
				pos2, ok2 := getPositionAtTime(path2, t)

				if ok1 && ok2 && pos1 == pos2 {
					return &Conflict{
						Robot1: robots[i],
						Robot2: robots[j],
						Vertex: pos1,
						Time:   t,
						IsEdge: false,
					}
				}
			}
		}
	}

	// Check for edge conflicts (swaps) between consecutive time points
	for ti := 0; ti < len(times)-1; ti++ {
		t1, t2 := times[ti], times[ti+1]

		for i := 0; i < len(robots); i++ {
			for j := i + 1; j < len(robots); j++ {
				path1, path2 := paths[robots[i]], paths[robots[j]]

				pos1Start, ok1s := getPositionAtTime(path1, t1)
				pos1End, ok1e := getPositionAtTime(path1, t2)
				pos2Start, ok2s := getPositionAtTime(path2, t1)
				pos2End, ok2e := getPositionAtTime(path2, t2)

				if ok1s && ok1e && ok2s && ok2e {
					// Check if robots swap positions
					if pos1Start == pos2End && pos1End == pos2Start && pos1Start != pos1End {
						return &Conflict{
							Robot1:   robots[i],
							Robot2:   robots[j],
							Vertex:   pos1Start,
							Time:     t1,
							IsEdge:   true,
							EdgeFrom: pos1Start,
							EdgeTo:   pos1End,
						}
					}
				}
			}
		}
	}

	return nil
}

// FindAllConflicts detects all conflicts in paths.
func FindAllConflicts(paths map[core.RobotID]core.Path) []*Conflict {
	var conflicts []*Conflict
	robots := sortedRobotIDs(paths)

	// Collect all unique time points
	timeSet := make(map[float64]bool)
	for _, path := range paths {
		for _, tv := range path {
			timeSet[tv.T] = true
		}
	}

	times := make([]float64, 0, len(timeSet))
	for t := range timeSet {
		times = append(times, t)
	}
	sort.Float64s(times)

	// Check for vertex conflicts
	for _, t := range times {
		for i := 0; i < len(robots); i++ {
			for j := i + 1; j < len(robots); j++ {
				path1, path2 := paths[robots[i]], paths[robots[j]]

				pos1, ok1 := getPositionAtTime(path1, t)
				pos2, ok2 := getPositionAtTime(path2, t)

				if ok1 && ok2 && pos1 == pos2 {
					conflicts = append(conflicts, &Conflict{
						Robot1: robots[i],
						Robot2: robots[j],
						Vertex: pos1,
						Time:   t,
						IsEdge: false,
					})
				}
			}
		}
	}

	// Check for edge conflicts
	for ti := 0; ti < len(times)-1; ti++ {
		t1, t2 := times[ti], times[ti+1]

		for i := 0; i < len(robots); i++ {
			for j := i + 1; j < len(robots); j++ {
				path1, path2 := paths[robots[i]], paths[robots[j]]

				pos1Start, ok1s := getPositionAtTime(path1, t1)
				pos1End, ok1e := getPositionAtTime(path1, t2)
				pos2Start, ok2s := getPositionAtTime(path2, t1)
				pos2End, ok2e := getPositionAtTime(path2, t2)

				if ok1s && ok1e && ok2s && ok2e {
					if pos1Start == pos2End && pos1End == pos2Start && pos1Start != pos1End {
						conflicts = append(conflicts, &Conflict{
							Robot1:   robots[i],
							Robot2:   robots[j],
							Vertex:   pos1Start,
							Time:     t1,
							IsEdge:   true,
							EdgeFrom: pos1Start,
							EdgeTo:   pos1End,
						})
					}
				}
			}
		}
	}

	return conflicts
}
