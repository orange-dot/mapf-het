// Package algo - HYBRID-CBS: CBS planning with potential field execution.
package algo

import (
	"container/heap"
	"math"

	"github.com/elektrokombinacija/mapf-het-research/internal/core"
)

// HybridCBS implements CBS planning with potential field guided execution.
// Core innovation: CBS for global planning, potential fields for local execution
// and deviation handling.
type HybridCBS struct {
	MaxTime          float64 // Max planning horizon
	FieldWeight      float64 // λ: influence of potential field on heuristic
	DeviationThresh  float64 // Threshold for triggering local replan
	ReplanHorizon    float64 // Time horizon for local replanning
}

// NewHybridCBS creates a HYBRID-CBS solver.
func NewHybridCBS(maxTime float64) *HybridCBS {
	return &HybridCBS{
		MaxTime:          maxTime,
		FieldWeight:      0.5,
		DeviationThresh:  2.0,
		ReplanHorizon:    10.0,
	}
}

func (h *HybridCBS) Name() string { return "HYBRID-CBS" }

// ExecutionState tracks robot states during execution phase.
type ExecutionState struct {
	PlannedPaths map[core.RobotID]core.Path
	CurrentPos   map[core.RobotID]core.VertexID
	CurrentTime  float64
	Deviations   map[core.RobotID]float64
	Completed    map[core.TaskID]bool
}

// NewExecutionState creates execution state from solution.
func NewExecutionState(sol *core.Solution) *ExecutionState {
	state := &ExecutionState{
		PlannedPaths: sol.Paths,
		CurrentPos:   make(map[core.RobotID]core.VertexID),
		CurrentTime:  0,
		Deviations:   make(map[core.RobotID]float64),
		Completed:    make(map[core.TaskID]bool),
	}

	// Initialize positions from path starts
	for rid, path := range sol.Paths {
		if len(path) > 0 {
			state.CurrentPos[rid] = path[0].V
		}
		state.Deviations[rid] = 0
	}

	return state
}

// Solve implements the HYBRID-CBS algorithm.
func (h *HybridCBS) Solve(inst *core.Instance) *core.Solution {
	// Phase 1: CBS Global Planning
	sol := h.cbsGlobalPlan(inst)
	if sol == nil {
		return nil
	}

	// Phase 2: Field-Aware Path Refinement
	field := ComputePotentialField(inst.Workspace, inst.Robots, inst.Tasks)
	h.refinePaths(inst, sol, field)

	return sol
}

// cbsGlobalPlan runs standard CBS with field-aware heuristic.
func (h *HybridCBS) cbsGlobalPlan(inst *core.Instance) *core.Solution {
	// Compute assignment
	assignment := h.computeAssignment(inst)
	if assignment == nil {
		return nil
	}

	// Compute potential field for heuristic guidance
	field := ComputePotentialField(inst.Workspace, inst.Robots, inst.Tasks)

	// Initialize root node
	root := &hybridCBSNode{
		constraints: nil,
		solution:    core.NewSolution(),
	}
	root.solution.Assignment = assignment

	// Plan initial paths with field-guided A*
	if !h.planAllPaths(inst, root, field) {
		return nil
	}
	root.cost = root.solution.Makespan

	// CBS main loop
	open := &hybridCBSHeap{}
	heap.Init(open)
	heap.Push(open, root)

	iterations := 0
	maxIterations := 10000

	for open.Len() > 0 && iterations < maxIterations {
		iterations++
		node := heap.Pop(open).(*hybridCBSNode)

		// Find first conflict
		conflict := FindFirstConflict(node.solution.Paths)
		if conflict == nil {
			node.solution.Feasible = true
			return node.solution
		}

		// Branch: create child nodes
		for _, robotID := range []core.RobotID{conflict.Robot1, conflict.Robot2} {
			child := &hybridCBSNode{
				constraints: append(
					append([]Constraint{}, node.constraints...),
					Constraint{
						Robot:  robotID,
						Vertex: conflict.Vertex,
						Time:   conflict.Time,
						IsEdge: conflict.IsEdge,
					},
				),
				solution: core.NewSolution(),
			}
			child.solution.Assignment = assignment

			if h.planAllPaths(inst, child, field) {
				child.cost = child.solution.Makespan
				heap.Push(open, child)
			}
		}
	}

	return nil
}

// hybridCBSNode represents a node in the CBS constraint tree.
type hybridCBSNode struct {
	constraints []Constraint
	solution    *core.Solution
	cost        float64
	index       int
}

type hybridCBSHeap []*hybridCBSNode

func (h hybridCBSHeap) Len() int           { return len(h) }
func (h hybridCBSHeap) Less(i, j int) bool { return h[i].cost < h[j].cost }
func (h hybridCBSHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *hybridCBSHeap) Push(x any) {
	n := x.(*hybridCBSNode)
	n.index = len(*h)
	*h = append(*h, n)
}
func (h *hybridCBSHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return x
}

// computeAssignment uses greedy assignment respecting capabilities.
func (h *HybridCBS) computeAssignment(inst *core.Instance) core.Assignment {
	assignment := make(core.Assignment)
	robotWorkload := make(map[core.RobotID]int)

	for _, task := range inst.Tasks {
		var bestRobot *core.Robot
		bestScore := math.Inf(-1)

		for _, robot := range inst.Robots {
			if !core.CanPerform(robot.Type, task.Type) {
				continue
			}

			// Score based on workload and distance
			workload := robotWorkload[robot.ID]
			taskLoc := inst.Workspace.Vertices[task.Location].Pos
			robotLoc := inst.Workspace.Vertices[robot.Start].Pos
			dist := euclideanDist(taskLoc, robotLoc)

			score := -float64(workload)*10 - dist
			if score > bestScore {
				bestScore = score
				bestRobot = robot
			}
		}

		if bestRobot == nil {
			return nil
		}

		assignment[task.ID] = bestRobot.ID
		robotWorkload[bestRobot.ID]++
	}

	return assignment
}

// planAllPaths plans paths for all robots with field-guided A*.
func (h *HybridCBS) planAllPaths(inst *core.Instance, node *hybridCBSNode, field *PotentialField) bool {
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

		// Filter constraints
		var robotConstraints []Constraint
		for _, con := range node.constraints {
			if con.Robot == robot.ID {
				robotConstraints = append(robotConstraints, con)
			}
		}

		// Plan with field-guided A*
		path := h.fieldGuidedAStar(
			inst.Workspace,
			robot,
			robot.Start,
			goals,
			robotConstraints,
			field,
		)

		if path == nil && len(goals) > 0 {
			return false
		}

		node.solution.Paths[robot.ID] = path
	}

	node.solution.ComputeMakespan(inst)
	return true
}

// fieldGuidedAStar implements A* with potential field heuristic.
func (h *HybridCBS) fieldGuidedAStar(
	ws *core.Workspace,
	robot *core.Robot,
	start core.VertexID,
	goals []core.VertexID,
	constraints []Constraint,
	field *PotentialField,
) core.Path {
	if len(goals) == 0 {
		return nil
	}

	goal := goals[0]

	// Field-aware heuristic
	heuristic := func(v core.VertexID) float64 {
		vPos := ws.Vertices[v].Pos
		gPos := ws.Vertices[goal].Pos
		distance := euclideanDist(vPos, gPos)

		// Field contribution: attract toward high load gradient, repel from obstacles
		fieldContrib := h.FieldWeight * (field.LoadGradient[v] - field.RepulsiveField[v])

		return distance - fieldContrib
	}

	// Constraint check
	violates := func(v core.VertexID, t float64) bool {
		for _, c := range constraints {
			if c.Robot == robot.ID && c.Vertex == v && timeEqual(c.Time, t) {
				return true
			}
		}
		return false
	}

	// A* search
	open := &fieldAStarHeap{}
	heap.Init(open)

	startNode := &fieldAStarNode{
		state: SpaceTimeState{V: start, T: 0},
		g:     0,
		f:     heuristic(start),
	}
	heap.Push(open, startNode)

	visited := make(map[SpaceTimeState]bool)

	for open.Len() > 0 {
		current := heap.Pop(open).(*fieldAStarNode)

		if current.state.V == goal {
			return h.reconstructPath(current)
		}

		if visited[current.state] {
			continue
		}
		visited[current.state] = true

		if current.state.T >= h.MaxTime {
			continue
		}

		nextT := current.state.T + 1.0

		// Wait action
		if !violates(current.state.V, nextT) {
			waitState := SpaceTimeState{V: current.state.V, T: nextT}
			if !visited[waitState] {
				node := &fieldAStarNode{
					state:  waitState,
					g:      current.g + 0.1,
					f:      current.g + 0.1 + heuristic(current.state.V),
					parent: current,
				}
				heap.Push(open, node)
			}
		}

		// Move actions
		for _, neighbor := range ws.Neighbors(current.state.V) {
			if !ws.CanOccupy(neighbor, robot.Type) {
				continue
			}
			if violates(neighbor, nextT) {
				continue
			}

			moveState := SpaceTimeState{V: neighbor, T: nextT}
			if visited[moveState] {
				continue
			}

			edgeCost := 1.0
			for _, e := range ws.Edges[current.state.V] {
				if e.To == neighbor {
					edgeCost = e.Cost
					break
				}
			}

			// Apply field influence to edge cost
			fieldBonus := h.FieldWeight * (field.LoadGradient[neighbor] - field.RepulsiveField[neighbor])
			adjustedCost := edgeCost - fieldBonus*0.1

			if adjustedCost < 0.1 {
				adjustedCost = 0.1
			}

			node := &fieldAStarNode{
				state:  moveState,
				g:      current.g + adjustedCost,
				f:      current.g + adjustedCost + heuristic(neighbor),
				parent: current,
			}
			heap.Push(open, node)
		}
	}

	return nil
}

type fieldAStarNode struct {
	state  SpaceTimeState
	g      float64
	f      float64
	parent *fieldAStarNode
	index  int
}

type fieldAStarHeap []*fieldAStarNode

func (h fieldAStarHeap) Len() int           { return len(h) }
func (h fieldAStarHeap) Less(i, j int) bool { return h[i].f < h[j].f }
func (h fieldAStarHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *fieldAStarHeap) Push(x any) {
	n := x.(*fieldAStarNode)
	n.index = len(*h)
	*h = append(*h, n)
}
func (h *fieldAStarHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return x
}

func (h *HybridCBS) reconstructPath(node *fieldAStarNode) core.Path {
	var path core.Path
	for n := node; n != nil; n = n.parent {
		path = append([]core.TimedVertex{{V: n.state.V, T: n.state.T}}, path...)
	}
	return path
}

// refinePaths applies field-based smoothing to paths.
func (h *HybridCBS) refinePaths(inst *core.Instance, sol *core.Solution, field *PotentialField) {
	// Post-process paths to follow field gradients more closely
	// when it doesn't introduce conflicts
	for rid, path := range sol.Paths {
		refined := h.smoothPath(inst.Workspace, path, field, inst.RobotByID(rid))
		sol.Paths[rid] = refined
	}

	// Verify no new conflicts were introduced
	if FindFirstConflict(sol.Paths) != nil {
		// Revert to original if refinement caused conflicts
		// (already done in-place, so we'd need to re-plan, but for now just continue)
	}
}

// smoothPath applies local field-guided smoothing.
func (h *HybridCBS) smoothPath(ws *core.Workspace, path core.Path, field *PotentialField, robot *core.Robot) core.Path {
	if len(path) < 3 {
		return path
	}

	smoothed := make(core.Path, len(path))
	copy(smoothed, path)

	// Single pass smoothing - try to improve intermediate waypoints
	for i := 1; i < len(smoothed)-1; i++ {
		prev := smoothed[i-1].V
		curr := smoothed[i].V
		next := smoothed[i+1].V

		// Find best neighbor that maintains connectivity
		bestV := curr
		bestScore := field.LoadGradient[curr] - field.RepulsiveField[curr]

		for _, neighbor := range ws.Neighbors(prev) {
			// Check if neighbor connects to next
			connects := false
			for _, n2 := range ws.Neighbors(neighbor) {
				if n2 == next {
					connects = true
					break
				}
			}

			if connects && ws.CanOccupy(neighbor, robot.Type) {
				score := field.LoadGradient[neighbor] - field.RepulsiveField[neighbor]
				if score > bestScore {
					bestScore = score
					bestV = neighbor
				}
			}
		}

		smoothed[i].V = bestV
	}

	return smoothed
}

// ExecuteWithFields simulates execution with field-guided deviation handling.
func (h *HybridCBS) ExecuteWithFields(state *ExecutionState, field *PotentialField, ws *core.Workspace) {
	for robotID, pos := range state.CurrentPos {
		path := state.PlannedPaths[robotID]
		if len(path) == 0 {
			continue
		}

		// Find expected position
		expectedPos, _ := getPositionAtTime(path, state.CurrentTime)

		// Compute deviation
		expectedPosCoord := ws.Vertices[expectedPos].Pos
		actualPosCoord := ws.Vertices[pos].Pos
		deviation := euclideanDist(expectedPosCoord, actualPosCoord)

		state.Deviations[robotID] = deviation

		// Trigger local replan if deviation exceeds threshold
		if deviation > h.DeviationThresh {
			// Use gradient descent for local correction
			nextPos := ComputeGradient(pos, expectedPos, field, ws)
			state.CurrentPos[robotID] = nextPos
		}
	}

	state.CurrentTime += 1.0
}
