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
type Edge struct {
	From, To VertexID
	Cost     float64 // Travel time
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

// AddEdge adds a bidirectional edge.
func (w *Workspace) AddEdge(from, to VertexID, cost float64) {
	w.Edges[from] = append(w.Edges[from], Edge{From: from, To: to, Cost: cost})
	w.Edges[to] = append(w.Edges[to], Edge{From: to, To: from, Cost: cost})
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
