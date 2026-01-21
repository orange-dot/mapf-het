// Package algo - 3D Space-Time A* for aerial drone pathfinding.
package algo

import (
	"container/heap"
	"math"

	"github.com/elektrokombinacija/mapf-het-research/internal/core"
)

// SpaceTimeState3D represents (vertex, time, layer) for 3D space-time A*.
type SpaceTimeState3D struct {
	V     core.VertexID
	T     float64
	Layer core.AirspaceLayer
}

// astar3DNode for priority queue.
type astar3DNode struct {
	state       SpaceTimeState3D
	g           float64 // Cost so far
	f           float64 // g + h
	energy      float64 // Remaining battery (for drones)
	parent      *astar3DNode
	index       int // heap index
	layerChange bool // Did we change layer to get here?
}

// astar3DHeap implements heap.Interface.
type astar3DHeap []*astar3DNode

func (h astar3DHeap) Len() int           { return len(h) }
func (h astar3DHeap) Less(i, j int) bool { return h[i].f < h[j].f }
func (h astar3DHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *astar3DHeap) Push(x any) {
	n := x.(*astar3DNode)
	n.index = len(*h)
	*h = append(*h, n)
}
func (h *astar3DHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return x
}

// SpaceTimeAStar3D finds shortest path for drones in 3D layered airspace.
// Considers energy constraints and vertical corridor transitions.
func SpaceTimeAStar3D(
	ws *core.Workspace,
	airspace *core.Airspace,
	robot *core.Robot,
	start core.VertexID,
	goals []core.VertexID,
	constraints []Constraint,
	maxTime float64,
) core.Path {
	if len(goals) == 0 {
		return nil
	}

	goal := goals[0]
	goalVertex := ws.Vertices[goal]
	if goalVertex == nil {
		return nil
	}

	startVertex := ws.Vertices[start]
	if startVertex == nil {
		return nil
	}

	// 3D Euclidean heuristic with energy penalty
	heuristic := func(state SpaceTimeState3D) float64 {
		pos := ws.Vertices[state.V].Pos
		goalPos := goalVertex.Pos

		dist3D := math.Sqrt(
			(pos.X-goalPos.X)*(pos.X-goalPos.X) +
				(pos.Y-goalPos.Y)*(pos.Y-goalPos.Y) +
				(pos.Z-goalPos.Z)*(pos.Z-goalPos.Z),
		)

		// Time estimate based on speed
		timeEst := dist3D / robot.Speed()

		return timeEst
	}

	// Energy-aware heuristic for low battery situations
	energyHeuristic := func(state SpaceTimeState3D, remainingEnergy float64) float64 {
		baseH := heuristic(state)

		// Add penalty if low battery and far from home
		if robot.IsDrone() && remainingEnergy < robot.BatteryCapacity*0.3 {
			homeVertex := ws.Vertices[robot.HomeBase]
			if homeVertex != nil {
				pos := ws.Vertices[state.V].Pos
				homePos := homeVertex.Pos
				homeDist := math.Sqrt(
					(pos.X-homePos.X)*(pos.X-homePos.X) +
						(pos.Y-homePos.Y)*(pos.Y-homePos.Y) +
						(pos.Z-homePos.Z)*(pos.Z-homePos.Z),
				)
				// Penalize being far from home when low on battery
				baseH += homeDist * 0.5
			}
		}

		return baseH
	}

	// Check if state violates constraints
	violates := func(v core.VertexID, t float64) bool {
		for _, c := range constraints {
			if c.Robot == robot.ID && c.Vertex == v && c.Time == t {
				return true
			}
		}
		return false
	}

	// Initialize
	open := &astar3DHeap{}
	heap.Init(open)

	initialEnergy := robot.CurrentBattery
	startNode := &astar3DNode{
		state: SpaceTimeState3D{
			V:     start,
			T:     0,
			Layer: startVertex.Layer,
		},
		g:      0,
		f:      heuristic(SpaceTimeState3D{V: start, T: 0, Layer: startVertex.Layer}),
		energy: initialEnergy,
	}
	heap.Push(open, startNode)

	visited := make(map[SpaceTimeState3D]bool)

	for open.Len() > 0 {
		current := heap.Pop(open).(*astar3DNode)

		// Goal check
		if current.state.V == goal {
			return reconstructPath3D(current)
		}

		if visited[current.state] {
			continue
		}
		visited[current.state] = true

		if current.state.T >= maxTime {
			continue
		}

		// Energy check - if depleted, path fails
		if robot.IsDrone() && current.energy <= 0 {
			continue
		}

		nextT := current.state.T + 1.0

		// === Action 1: Hover (wait in place) ===
		if !violates(current.state.V, nextT) {
			hoverEnergy := robot.EnergyForDistance(0, core.ActionHover) * 1.0 // 1 second hover
			newEnergy := current.energy - hoverEnergy

			if !robot.IsDrone() || newEnergy > 0 {
				hoverState := SpaceTimeState3D{
					V:     current.state.V,
					T:     nextT,
					Layer: current.state.Layer,
				}
				if !visited[hoverState] {
					node := &astar3DNode{
						state:  hoverState,
						g:      current.g + 0.1,
						f:      current.g + 0.1 + energyHeuristic(hoverState, newEnergy),
						energy: newEnergy,
						parent: current,
					}
					heap.Push(open, node)
				}
			}
		}

		// === Action 2: Horizontal movement (same layer) ===
		for _, neighbor := range ws.Neighbors(current.state.V) {
			neighborVertex := ws.Vertices[neighbor]
			if neighborVertex == nil {
				continue
			}

			// Only same-layer horizontal moves here
			if neighborVertex.Layer != current.state.Layer {
				continue
			}

			// Check occupancy restrictions
			if !ws.CanOccupy(neighbor, robot.Type) {
				continue
			}

			// Check no-fly zones for drones
			if robot.IsDrone() && neighborVertex.NoFlyZone {
				continue
			}

			if violates(neighbor, nextT) {
				continue
			}

			// Calculate edge cost and energy
			edgeCost := 1.0
			for _, e := range ws.Edges[current.state.V] {
				if e.To == neighbor {
					edgeCost = e.Cost
					break
				}
			}

			// Energy for horizontal movement
			dist := edgeCost // Assuming edge cost ~ distance
			moveEnergy := robot.EnergyForDistance(dist, core.ActionMoveHorizontal)
			newEnergy := current.energy - moveEnergy

			if robot.IsDrone() && newEnergy <= 0 {
				continue
			}

			moveState := SpaceTimeState3D{
				V:     neighbor,
				T:     nextT,
				Layer: current.state.Layer,
			}
			if visited[moveState] {
				continue
			}

			node := &astar3DNode{
				state:  moveState,
				g:      current.g + edgeCost,
				f:      current.g + edgeCost + energyHeuristic(moveState, newEnergy),
				energy: newEnergy,
				parent: current,
			}
			heap.Push(open, node)
		}

		// === Action 3: Vertical movement (layer changes) ===
		// Only for drones and only in corridors
		if robot.IsDrone() {
			currentVertex := ws.Vertices[current.state.V]
			if currentVertex != nil && currentVertex.IsCorridor && airspace != nil {
				// Try climbing
				upperLayer := core.NextLayerUp(current.state.Layer)
				if upperLayer != current.state.Layer {
					upperVertex := airspace.GetVertexAtLayer(current.state.V, upperLayer)
					if upperVertex != 0 && !violates(upperVertex, nextT+1) {
						climbEnergy := robot.EnergyForLayerChange(current.state.Layer, upperLayer)
						newEnergy := current.energy - climbEnergy

						if newEnergy > 0 {
							climbState := SpaceTimeState3D{
								V:     upperVertex,
								T:     nextT + 1, // Climbing takes extra time
								Layer: upperLayer,
							}
							if !visited[climbState] {
								node := &astar3DNode{
									state:       climbState,
									g:           current.g + 2.0, // Layer change cost
									f:           current.g + 2.0 + energyHeuristic(climbState, newEnergy),
									energy:      newEnergy,
									parent:      current,
									layerChange: true,
								}
								heap.Push(open, node)
							}
						}
					}
				}

				// Try descending
				lowerLayer := core.NextLayerDown(current.state.Layer)
				if lowerLayer != current.state.Layer {
					lowerVertex := airspace.GetVertexAtLayer(current.state.V, lowerLayer)
					if lowerVertex != 0 && !violates(lowerVertex, nextT) {
						descendEnergy := robot.EnergyForLayerChange(current.state.Layer, lowerLayer)
						newEnergy := current.energy - descendEnergy

						if newEnergy > 0 {
							descendState := SpaceTimeState3D{
								V:     lowerVertex,
								T:     nextT,
								Layer: lowerLayer,
							}
							if !visited[descendState] {
								node := &astar3DNode{
									state:       descendState,
									g:           current.g + 1.0, // Descending is faster
									f:           current.g + 1.0 + energyHeuristic(descendState, newEnergy),
									energy:      newEnergy,
									parent:      current,
									layerChange: true,
								}
								heap.Push(open, node)
							}
						}
					}
				}
			}
		}

		// === Action 4: Recharge at pad ===
		if robot.IsDrone() {
			currentVertex := ws.Vertices[current.state.V]
			if currentVertex != nil && currentVertex.IsPad && current.state.Layer == core.LayerGround {
				// Recharge to full - takes 10 time units
				rechargeState := SpaceTimeState3D{
					V:     current.state.V,
					T:     current.state.T + 10.0,
					Layer: core.LayerGround,
				}
				if !visited[rechargeState] && !violates(current.state.V, rechargeState.T) {
					node := &astar3DNode{
						state:  rechargeState,
						g:      current.g + 10.0,
						f:      current.g + 10.0 + energyHeuristic(rechargeState, robot.BatteryCapacity),
						energy: robot.BatteryCapacity,
						parent: current,
					}
					heap.Push(open, node)
				}
			}
		}
	}

	return nil // No path found
}

// reconstructPath3D builds path from A* result.
func reconstructPath3D(node *astar3DNode) core.Path {
	var path core.Path
	for n := node; n != nil; n = n.parent {
		path = append([]core.TimedVertex{{V: n.state.V, T: n.state.T}}, path...)
	}
	return path
}

// euclideanDist3D computes 3D Euclidean distance.
func euclideanDist3D(p1, p2 core.Pos) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	dz := p1.Z - p2.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
