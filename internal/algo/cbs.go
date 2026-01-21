package algo

import (
	"container/heap"

	"github.com/elektrokombinacija/mapf-het-research/internal/core"
)

// CBS implements Conflict-Based Search for MAPF-HET.
type CBS struct {
	MaxTime float64 // Max planning horizon
}

// NewCBS creates a CBS solver.
func NewCBS(maxTime float64) *CBS {
	return &CBS{MaxTime: maxTime}
}

func (c *CBS) Name() string { return "CBS-HET" }

// cbsNode represents a node in the CBS constraint tree.
type cbsNode struct {
	constraints []Constraint
	solution    *core.Solution
	cost        float64
	index       int
}

type cbsHeap []*cbsNode

func (h cbsHeap) Len() int           { return len(h) }
func (h cbsHeap) Less(i, j int) bool { return h[i].cost < h[j].cost }
func (h cbsHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *cbsHeap) Push(x any) {
	n := x.(*cbsNode)
	n.index = len(*h)
	*h = append(*h, n)
}
func (h *cbsHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return x
}

// Solve implements the CBS algorithm.
func (c *CBS) Solve(inst *core.Instance) *core.Solution {
	// Step 1: Compute initial assignment (greedy)
	assignment := c.computeAssignment(inst)
	if assignment == nil {
		return nil
	}

	// Step 2: Initialize root node
	root := &cbsNode{
		constraints: nil,
		solution:    core.NewSolution(),
	}
	root.solution.Assignment = assignment

	// Plan initial paths
	if !c.planAllPaths(inst, root) {
		return nil
	}
	root.cost = root.solution.Makespan

	// Step 3: CBS main loop
	open := &cbsHeap{}
	heap.Init(open)
	heap.Push(open, root)

	for open.Len() > 0 {
		node := heap.Pop(open).(*cbsNode)

		// Find first conflict
		conflict := FindFirstConflict(node.solution.Paths)
		if conflict == nil {
			// No conflicts - solution found
			node.solution.Feasible = true
			return node.solution
		}

		// Branch: create child nodes with new constraints
		for _, robotID := range []core.RobotID{conflict.Robot1, conflict.Robot2} {
			child := &cbsNode{
				constraints: append(
					append([]Constraint{}, node.constraints...),
					Constraint{
						Robot:  robotID,
						Vertex: conflict.Vertex,
						Time:   conflict.Time,
					},
				),
				solution: core.NewSolution(),
			}
			child.solution.Assignment = assignment

			// Re-plan path for constrained robot
			if c.planAllPaths(inst, child) {
				child.cost = child.solution.Makespan
				heap.Push(open, child)
			}
		}
	}

	return nil // No solution found
}

// computeAssignment uses greedy assignment respecting capabilities.
func (c *CBS) computeAssignment(inst *core.Instance) core.Assignment {
	assignment := make(core.Assignment)

	for _, task := range inst.Tasks {
		assigned := false
		for _, robot := range inst.Robots {
			if core.CanPerform(robot.Type, task.Type) {
				assignment[task.ID] = robot.ID
				assigned = true
				break
			}
		}
		if !assigned {
			return nil // No capable robot
		}
	}

	return assignment
}

// planAllPaths plans paths for all robots.
func (c *CBS) planAllPaths(inst *core.Instance, node *cbsNode) bool {
	node.solution.Paths = make(map[core.RobotID]core.Path)
	node.solution.Schedule = make(core.Schedule)

	for _, robot := range inst.Robots {
		// Collect tasks for this robot
		var goals []core.VertexID
		for tid, rid := range node.solution.Assignment {
			if rid == robot.ID {
				task := inst.TaskByID(tid)
				if task != nil {
					goals = append(goals, task.Location)
				}
			}
		}

		// Filter constraints for this robot
		var robotConstraints []Constraint
		for _, con := range node.constraints {
			if con.Robot == robot.ID {
				robotConstraints = append(robotConstraints, con)
			}
		}

		// Plan path
		path := SpaceTimeAStar(
			inst.Workspace,
			robot,
			robot.Start,
			goals,
			robotConstraints,
			c.MaxTime,
		)

		if path == nil && len(goals) > 0 {
			return false
		}

		node.solution.Paths[robot.ID] = path
	}

	node.solution.ComputeMakespan(inst)
	return true
}
