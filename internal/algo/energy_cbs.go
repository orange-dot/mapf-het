// Package algo - ENERGY-CBS: CBS with battery constraints for aerial drones.
package algo

import (
	"container/heap"
	"math"

	"github.com/elektrokombinacija/mapf-het-research/internal/core"
)

// EnergyCBS extends CBS with battery constraints for TypeC drones.
// Ensures drones reach charging pads before battery depletes.
type EnergyCBS struct {
	MaxTime  float64
	Airspace *core.Airspace
}

// NewEnergyCBS creates an ENERGY-CBS solver.
func NewEnergyCBS(maxTime float64) *EnergyCBS {
	return &EnergyCBS{
		MaxTime:  maxTime,
		Airspace: nil,
	}
}

func (e *EnergyCBS) Name() string { return "ENERGY-CBS" }

// EnergyConstraint forces drone to reach charging pad by specified time.
type EnergyConstraint struct {
	Robot       core.RobotID
	MustReachBy float64       // Time by which must reach pad
	Pad         core.VertexID // Which charging pad
}

// energyCBSNode represents a node in the constraint tree.
type energyCBSNode struct {
	constraints       []Constraint
	energyConstraints []EnergyConstraint
	solution          *core.Solution
	cost              float64
	index             int
}

type energyCBSHeap []*energyCBSNode

func (h energyCBSHeap) Len() int           { return len(h) }
func (h energyCBSHeap) Less(i, j int) bool { return h[i].cost < h[j].cost }
func (h energyCBSHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *energyCBSHeap) Push(x any) {
	n := x.(*energyCBSNode)
	n.index = len(*h)
	*h = append(*h, n)
}
func (h *energyCBSHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return x
}

// Solve implements the ENERGY-CBS algorithm.
func (e *EnergyCBS) Solve(inst *core.Instance) *core.Solution {
	// Build airspace
	e.Airspace = core.BuildAirspace(inst.Workspace)

	// Compute assignment
	assignment := e.computeAssignment(inst)
	if assignment == nil {
		return nil
	}

	// Initialize root
	root := &energyCBSNode{
		constraints:       nil,
		energyConstraints: nil,
		solution:          core.NewSolution(),
	}
	root.solution.Assignment = assignment

	if !e.planAllPaths(inst, root) {
		return nil
	}
	root.cost = root.solution.Makespan

	// CBS main loop
	open := &energyCBSHeap{}
	heap.Init(open)
	heap.Push(open, root)

	iterations := 0
	maxIterations := 10000

	for open.Len() > 0 && iterations < maxIterations {
		iterations++
		node := heap.Pop(open).(*energyCBSNode)

		// Check energy feasibility for all drones
		energyViolation := e.checkEnergyFeasibility(inst, node)
		if energyViolation != nil {
			// Add charging waypoint constraint
			children := e.resolveEnergyViolation(inst, node, energyViolation)
			for _, child := range children {
				if e.planAllPaths(inst, child) {
					child.cost = child.solution.Makespan
					heap.Push(open, child)
				}
			}
			continue
		}

		// Find spatial conflicts
		conflict := FindFirstConflict(node.solution.Paths)
		if conflict == nil {
			node.solution.Feasible = true
			return node.solution
		}

		// Standard CBS branching for spatial conflicts
		children := e.resolveConflict(node, conflict)
		for _, child := range children {
			if e.planAllPaths(inst, child) {
				child.cost = child.solution.Makespan
				heap.Push(open, child)
			}
		}
	}

	return nil
}

// EnergyViolation represents a drone that will run out of battery.
type EnergyViolation struct {
	Robot      *core.Robot
	DepletedAt float64       // Time when battery depletes
	Position   core.VertexID // Position at depletion
}

// checkEnergyFeasibility verifies all drone paths are energy-feasible.
func (e *EnergyCBS) checkEnergyFeasibility(inst *core.Instance, node *energyCBSNode) *EnergyViolation {
	for _, robot := range inst.Robots {
		if robot.Type != core.TypeC {
			continue
		}

		path := node.solution.Paths[robot.ID]
		if path == nil {
			continue
		}

		violation := e.simulateEnergy(inst, robot, path)
		if violation != nil {
			return violation
		}
	}
	return nil
}

// simulateEnergy simulates battery consumption along path.
func (e *EnergyCBS) simulateEnergy(inst *core.Instance, robot *core.Robot, path core.Path) *EnergyViolation {
	energy := robot.CurrentBattery

	for i := 1; i < len(path); i++ {
		prev := inst.Workspace.Vertices[path[i-1].V]
		curr := inst.Workspace.Vertices[path[i].V]

		if prev == nil || curr == nil {
			continue
		}

		// Calculate energy for this segment
		dist := euclideanDist3D(prev.Pos, curr.Pos)

		// Determine action type
		var action core.MoveAction
		if prev.Layer != curr.Layer {
			if curr.Layer > prev.Layer {
				action = core.ActionClimb
			} else {
				action = core.ActionDescend
			}
		} else if dist < 0.01 {
			action = core.ActionHover
		} else {
			action = core.ActionMoveHorizontal
		}

		// Calculate consumption
		consumption := robot.EnergyForDistance(dist, action)
		if prev.Layer != curr.Layer {
			consumption += robot.EnergyForLayerChange(prev.Layer, curr.Layer)
		}

		energy -= consumption

		// Check if at charging pad - recharge
		if curr.IsPad && curr.Layer == core.LayerGround {
			energy = robot.BatteryCapacity
		}

		// Check depletion
		if energy <= 0 {
			return &EnergyViolation{
				Robot:      robot,
				DepletedAt: path[i].T,
				Position:   path[i].V,
			}
		}
	}

	return nil
}

// resolveEnergyViolation adds charging waypoint for depleted drone.
func (e *EnergyCBS) resolveEnergyViolation(inst *core.Instance, node *energyCBSNode, violation *EnergyViolation) []*energyCBSNode {
	var children []*energyCBSNode

	// Find nearest charging pad
	nearestPad := e.findNearestPad(inst, violation.Position)
	if nearestPad == 0 {
		return children
	}

	// Add energy constraint: must reach pad before depletion
	// Give some buffer time
	reachBy := violation.DepletedAt - 5.0
	if reachBy < 0 {
		reachBy = 0
	}

	child := &energyCBSNode{
		constraints:       append([]Constraint{}, node.constraints...),
		energyConstraints: append([]EnergyConstraint{}, node.energyConstraints...),
		solution:          core.NewSolution(),
	}
	child.solution.Assignment = node.solution.Assignment
	child.energyConstraints = append(child.energyConstraints, EnergyConstraint{
		Robot:       violation.Robot.ID,
		MustReachBy: reachBy,
		Pad:         nearestPad,
	})

	children = append(children, child)

	return children
}

// findNearestPad finds closest charging pad to a position.
func (e *EnergyCBS) findNearestPad(inst *core.Instance, pos core.VertexID) core.VertexID {
	if e.Airspace != nil {
		return e.Airspace.GetNearestPad(inst.Workspace, pos)
	}

	// Fallback: search workspace
	posVertex := inst.Workspace.Vertices[pos]
	if posVertex == nil {
		return 0
	}

	nearestPad := core.VertexID(0)
	nearestDist := math.Inf(1)

	for vid, v := range inst.Workspace.Vertices {
		if v.IsPad {
			dist := euclideanDist3D(posVertex.Pos, v.Pos)
			if dist < nearestDist {
				nearestDist = dist
				nearestPad = vid
			}
		}
	}

	return nearestPad
}

// resolveConflict generates child nodes with standard CBS branching.
func (e *EnergyCBS) resolveConflict(node *energyCBSNode, c *Conflict) []*energyCBSNode {
	var children []*energyCBSNode

	for _, robotID := range []core.RobotID{c.Robot1, c.Robot2} {
		child := &energyCBSNode{
			constraints: append(
				append([]Constraint{}, node.constraints...),
				Constraint{
					Robot:  robotID,
					Vertex: c.Vertex,
					Time:   c.Time,
				},
			),
			energyConstraints: append([]EnergyConstraint{}, node.energyConstraints...),
			solution:          core.NewSolution(),
		}
		child.solution.Assignment = node.solution.Assignment
		children = append(children, child)
	}

	return children
}

// computeAssignment assigns tasks to robots.
func (e *EnergyCBS) computeAssignment(inst *core.Instance) core.Assignment {
	assignment := make(core.Assignment)
	robotWorkload := make(map[core.RobotID]int)

	for _, task := range inst.Tasks {
		var bestRobot *core.Robot
		bestScore := math.Inf(-1)

		for _, robot := range inst.Robots {
			if !core.CanPerform(robot.Type, task.Type) {
				continue
			}

			workload := robotWorkload[robot.ID]
			taskLoc := inst.Workspace.Vertices[task.Location].Pos
			robotLoc := inst.Workspace.Vertices[robot.Start].Pos
			dist := euclideanDist3D(taskLoc, robotLoc)

			// For drones, prefer those with more battery
			batteryBonus := 0.0
			if robot.Type == core.TypeC {
				batteryBonus = robot.BatteryPercentage() / 20.0
			}

			score := -float64(workload)*10 - dist + batteryBonus
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

// planAllPaths plans paths for all robots with energy constraints.
func (e *EnergyCBS) planAllPaths(inst *core.Instance, node *energyCBSNode) bool {
	node.solution.Paths = make(map[core.RobotID]core.Path)
	node.solution.Schedule = make(core.Schedule)

	for _, robot := range inst.Robots {
		var goals []core.VertexID

		// Add task goals
		for tid, rid := range node.solution.Assignment {
			if rid == robot.ID {
				task := inst.TaskByID(tid)
				if task != nil {
					goals = append(goals, task.Location)
				}
			}
		}

		// For drones, check energy constraints and add charging waypoints
		if robot.Type == core.TypeC {
			for _, ec := range node.energyConstraints {
				if ec.Robot == robot.ID {
					// Insert charging pad as intermediate goal
					goals = append([]core.VertexID{ec.Pad}, goals...)
				}
			}
		}

		var robotConstraints []Constraint
		for _, con := range node.constraints {
			if con.Robot == robot.ID {
				robotConstraints = append(robotConstraints, con)
			}
		}

		var path core.Path
		if robot.Type == core.TypeC && e.Airspace != nil {
			// Use 3D A* for drones
			path = SpaceTimeAStar3D(
				inst.Workspace,
				e.Airspace,
				robot,
				robot.Start,
				goals,
				robotConstraints,
				e.MaxTime,
			)
		} else {
			// Use standard A* for ground robots
			path = SpaceTimeAStar(
				inst.Workspace,
				robot,
				robot.Start,
				goals,
				robotConstraints,
				e.MaxTime,
			)
		}

		if path == nil && len(goals) > 0 {
			return false
		}

		node.solution.Paths[robot.ID] = path
	}

	node.solution.ComputeMakespan(inst)
	return true
}
