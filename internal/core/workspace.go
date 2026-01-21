package core

// Pos represents a 3D position (Z=0 for ground robots).
type Pos struct {
	X, Y, Z float64
}

// AirspaceLayer defines discrete altitude layers for drones.
type AirspaceLayer int

const (
	LayerGround AirspaceLayer = 0  // Z = 0m (ground level)
	Layer1      AirspaceLayer = 5  // Z = 5m (handoff layer)
	Layer2      AirspaceLayer = 10 // Z = 10m (work layer)
	Layer3      AirspaceLayer = 15 // Z = 15m (transit layer)
)

// AllLayers returns all valid airspace layers.
func AllLayers() []AirspaceLayer {
	return []AirspaceLayer{LayerGround, Layer1, Layer2, Layer3}
}

// Height returns the altitude in meters for a layer.
func (l AirspaceLayer) Height() float64 {
	return float64(l)
}

// VertexID is a unique vertex identifier.
type VertexID int

// Vertex represents a location in the workspace graph.
type Vertex struct {
	ID       VertexID
	Pos      Pos
	Shared   bool        // Multiple robots can occupy simultaneously
	Restrict []RobotType // If non-empty, only these types allowed
	// Airspace properties (for TypeC drones)
	Layer     AirspaceLayer // Altitude layer (0 for ground vertices)
	IsCorridor bool         // Vertical corridor (takeoff/landing allowed)
	IsPad     bool          // Charging/landing pad
	NoFlyZone bool          // Restricted airspace
}

// Edge connects two vertices.
// Semantics:
//   - LengthMeters: physical distance in meters (required)
//   - TravelTimeSec: optional fixed traversal time in seconds (for elevators, rails, etc.)
//     If TravelTimeSec > 0, it is authoritative and robot speed is ignored.
//     If TravelTimeSec == 0, travel time = LengthMeters / robot.Speed().
type Edge struct {
	From, To      VertexID
	LengthMeters  float64 // Physical distance in meters
	TravelTimeSec float64 // Fixed travel time (0 = use length/speed)

	// Deprecated: use LengthMeters. Kept for backward compatibility.
	Cost float64
}

// TravelTime returns the traversal time for this edge given a robot.
// Priority: TravelTimeSec > LengthMeters/Speed > Cost (legacy fallback).
func (e Edge) TravelTime(robot *Robot) float64 {
	// Priority 1: explicit fixed time
	if e.TravelTimeSec > 0 {
		return e.TravelTimeSec
	}

	// Priority 2: length-based calculation
	if e.LengthMeters > 0 && robot != nil && robot.Speed() > 0 {
		return e.LengthMeters / robot.Speed()
	}

	// Fallback: legacy Cost field (treat as distance, divide by speed)
	if e.Cost > 0 && robot != nil && robot.Speed() > 0 {
		return e.Cost / robot.Speed()
	}

	// Default: 1 second if nothing else
	return 1.0
}

// Distance returns the physical distance for this edge.
// Priority: LengthMeters > Cost (legacy) > 0.
func (e Edge) Distance() float64 {
	if e.LengthMeters > 0 {
		return e.LengthMeters
	}
	if e.Cost > 0 {
		return e.Cost
	}
	return 0
}

// Workspace represents the traversable space.
type Workspace struct {
	Vertices map[VertexID]*Vertex
	Edges    map[VertexID][]Edge // Adjacency list
}

// NewWorkspace creates an empty workspace.
func NewWorkspace() *Workspace {
	return &Workspace{
		Vertices: make(map[VertexID]*Vertex),
		Edges:    make(map[VertexID][]Edge),
	}
}

// AddVertex adds a vertex to the workspace.
func (w *Workspace) AddVertex(v *Vertex) {
	w.Vertices[v.ID] = v
	if w.Edges[v.ID] == nil {
		w.Edges[v.ID] = []Edge{}
	}
}

// AddEdge adds a bidirectional edge (legacy: cost treated as distance in meters).
func (w *Workspace) AddEdge(from, to VertexID, cost float64) {
	// Set both Cost (legacy) and LengthMeters for backward compatibility
	w.Edges[from] = append(w.Edges[from], Edge{From: from, To: to, Cost: cost, LengthMeters: cost})
	w.Edges[to] = append(w.Edges[to], Edge{From: to, To: from, Cost: cost, LengthMeters: cost})
}

// AddEdgeWithLength adds a bidirectional edge with explicit length.
func (w *Workspace) AddEdgeWithLength(from, to VertexID, lengthMeters float64) {
	w.Edges[from] = append(w.Edges[from], Edge{From: from, To: to, LengthMeters: lengthMeters})
	w.Edges[to] = append(w.Edges[to], Edge{From: to, To: from, LengthMeters: lengthMeters})
}

// AddEdgeWithFixedTime adds a bidirectional edge with fixed traversal time (e.g., elevator).
func (w *Workspace) AddEdgeWithFixedTime(from, to VertexID, lengthMeters, travelTimeSec float64) {
	w.Edges[from] = append(w.Edges[from], Edge{From: from, To: to, LengthMeters: lengthMeters, TravelTimeSec: travelTimeSec})
	w.Edges[to] = append(w.Edges[to], Edge{From: to, To: from, LengthMeters: lengthMeters, TravelTimeSec: travelTimeSec})
}

// GetEdge returns the edge between two vertices, or nil if none exists.
func (w *Workspace) GetEdge(from, to VertexID) *Edge {
	for i := range w.Edges[from] {
		if w.Edges[from][i].To == to {
			return &w.Edges[from][i]
		}
	}
	return nil
}

// Neighbors returns adjacent vertices.
func (w *Workspace) Neighbors(v VertexID) []VertexID {
	edges := w.Edges[v]
	neighbors := make([]VertexID, len(edges))
	for i, e := range edges {
		neighbors[i] = e.To
	}
	return neighbors
}

// CanOccupy checks if robot type can occupy vertex.
func (w *Workspace) CanOccupy(v VertexID, rt RobotType) bool {
	vertex := w.Vertices[v]
	if vertex == nil {
		return false
	}
	if len(vertex.Restrict) == 0 {
		return true
	}
	for _, allowed := range vertex.Restrict {
		if allowed == rt {
			return true
		}
	}
	return false
}
