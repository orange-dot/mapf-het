package algo

import (
	"container/heap"

	"github.com/elektrokombinacija/mapf-het-research/internal/core"
)

// SpaceTimeState represents (vertex, time) for space-time A*.
type SpaceTimeState struct {
	V core.VertexID
	T float64
}

// astarNode for priority queue.
type astarNode struct {
	state  SpaceTimeState
	g      float64 // Cost so far
	f      float64 // g + h
	parent *astarNode
	index  int // heap index
}

// astarHeap implements heap.Interface.
type astarHeap []*astarNode

func (h astarHeap) Len() int           { return len(h) }
func (h astarHeap) Less(i, j int) bool { return h[i].f < h[j].f }
func (h astarHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *astarHeap) Push(x any) {
	n := x.(*astarNode)
	n.index = len(*h)
	*h = append(*h, n)
}
func (h *astarHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return x
}

// SpaceTimeAStar finds shortest path avoiding constraints.
func SpaceTimeAStar(
	ws *core.Workspace,
	robot *core.Robot,
	start core.VertexID,
	goals []core.VertexID,
	constraints []Constraint,
	maxTime float64,
) core.Path {
	if len(goals) == 0 {
		return nil
	}

	// For now: single goal A*
	goal := goals[0]

	// Heuristic: Manhattan distance (placeholder)
	heuristic := func(v core.VertexID) float64 {
		// TODO: Proper heuristic using workspace positions
		if v == goal {
			return 0
		}
		return 1.0
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

	open := &astarHeap{}
	heap.Init(open)

	startNode := &astarNode{
		state: SpaceTimeState{V: start, T: 0},
		g:     0,
		f:     heuristic(start),
	}
	heap.Push(open, startNode)

	visited := make(map[SpaceTimeState]bool)

	for open.Len() > 0 {
		current := heap.Pop(open).(*astarNode)

		if current.state.V == goal {
			// Reconstruct path
			return reconstructPath(current)
		}

		if visited[current.state] {
			continue
		}
		visited[current.state] = true

		if current.state.T >= maxTime {
			continue
		}

		// Expand neighbors
		nextT := current.state.T + 1.0 // Unit time step

		// Wait action
		if !violates(current.state.V, nextT) {
			waitState := SpaceTimeState{V: current.state.V, T: nextT}
			if !visited[waitState] {
				node := &astarNode{
					state:  waitState,
					g:      current.g + 0.1, // Small cost for waiting
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

			// Get edge cost
			edgeCost := 1.0
			for _, e := range ws.Edges[current.state.V] {
				if e.To == neighbor {
					edgeCost = e.Cost
					break
				}
			}

			node := &astarNode{
				state:  moveState,
				g:      current.g + edgeCost,
				f:      current.g + edgeCost + heuristic(neighbor),
				parent: current,
			}
			heap.Push(open, node)
		}
	}

	return nil // No path found
}

func reconstructPath(node *astarNode) core.Path {
	var path core.Path
	for n := node; n != nil; n = n.parent {
		path = append([]core.TimedVertex{{V: n.state.V, T: n.state.T}}, path...)
	}
	return path
}
