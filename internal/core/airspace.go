package core

// VerticalCorridor represents a vertical path for drone takeoff/landing.
type VerticalCorridor struct {
	ID            int
	BaseVertex    VertexID                    // Ground pad vertex
	LayerVertices map[AirspaceLayer]VertexID // Vertex at each layer
}

// Airspace manages layered drone navigation.
type Airspace struct {
	Layers     map[AirspaceLayer][]*Vertex  // Vertices per layer
	Corridors  []*VerticalCorridor          // Takeoff/landing points
	LayerGraph map[AirspaceLayer]*Workspace // 2D graph per layer
}

// NewAirspace creates an empty airspace.
func NewAirspace() *Airspace {
	return &Airspace{
		Layers:     make(map[AirspaceLayer][]*Vertex),
		Corridors:  nil,
		LayerGraph: make(map[AirspaceLayer]*Workspace),
	}
}

// BuildAirspace constructs airspace from workspace vertices.
func BuildAirspace(ws *Workspace) *Airspace {
	a := NewAirspace()

	// Group vertices by layer
	for _, v := range ws.Vertices {
		a.Layers[v.Layer] = append(a.Layers[v.Layer], v)
	}

	// Build corridors from pad vertices
	corridorID := 0
	padVertices := make(map[VertexID]bool)

	// Find ground pads
	for _, v := range ws.Vertices {
		if v.IsPad && v.Layer == LayerGround {
			padVertices[v.ID] = true
		}
	}

	// For each pad, find corridor vertices above it
	for padID := range padVertices {
		padVertex := ws.Vertices[padID]
		corridor := &VerticalCorridor{
			ID:            corridorID,
			BaseVertex:    padID,
			LayerVertices: make(map[AirspaceLayer]VertexID),
		}
		corridor.LayerVertices[LayerGround] = padID

		// Find vertices at each layer that are corridor-aligned
		for _, v := range ws.Vertices {
			if v.IsCorridor && v.Layer != LayerGround {
				// Check if aligned (same X, Y position)
				if v.Pos.X == padVertex.Pos.X && v.Pos.Y == padVertex.Pos.Y {
					corridor.LayerVertices[v.Layer] = v.ID
				}
			}
		}

		if len(corridor.LayerVertices) > 1 {
			a.Corridors = append(a.Corridors, corridor)
			corridorID++
		}
	}

	// Build layer graphs (2D navigation within each layer)
	for layer := range a.Layers {
		a.LayerGraph[layer] = NewWorkspace()
		for _, v := range a.Layers[layer] {
			a.LayerGraph[layer].AddVertex(v)
		}
	}

	// Copy edges that connect vertices in the same layer
	for _, edges := range ws.Edges {
		for _, e := range edges {
			fromV := ws.Vertices[e.From]
			toV := ws.Vertices[e.To]
			if fromV.Layer == toV.Layer {
				if _, exists := a.LayerGraph[fromV.Layer].Vertices[e.From]; exists {
					if _, exists := a.LayerGraph[fromV.Layer].Vertices[e.To]; exists {
						// Only add if not already present
						hasEdge := false
						for _, existing := range a.LayerGraph[fromV.Layer].Edges[e.From] {
							if existing.To == e.To {
								hasEdge = true
								break
							}
						}
						if !hasEdge {
							a.LayerGraph[fromV.Layer].Edges[e.From] = append(
								a.LayerGraph[fromV.Layer].Edges[e.From],
								Edge{From: e.From, To: e.To, Cost: e.Cost},
							)
						}
					}
				}
			}
		}
	}

	return a
}

// CanTransition checks if drone can move between layers at given vertex.
func (a *Airspace) CanTransition(from, to AirspaceLayer, pos VertexID) bool {
	// Can only transition in corridors
	for _, c := range a.Corridors {
		// Check if position is in this corridor
		for _, vid := range c.LayerVertices {
			if vid == pos {
				// Check if both layers exist in corridor
				_, fromExists := c.LayerVertices[from]
				_, toExists := c.LayerVertices[to]
				return fromExists && toExists
			}
		}
	}
	return false
}

// GetVertexAtLayer returns the vertex ID at a specific layer in a corridor.
// Returns 0 if not found.
func (a *Airspace) GetVertexAtLayer(corridorPos VertexID, layer AirspaceLayer) VertexID {
	for _, c := range a.Corridors {
		// Check if corridorPos is part of this corridor
		for _, vid := range c.LayerVertices {
			if vid == corridorPos {
				if targetVid, exists := c.LayerVertices[layer]; exists {
					return targetVid
				}
				return 0
			}
		}
	}
	return 0
}

// GetCorridorForVertex returns the corridor containing this vertex, or nil.
func (a *Airspace) GetCorridorForVertex(vid VertexID) *VerticalCorridor {
	for _, c := range a.Corridors {
		for _, cVid := range c.LayerVertices {
			if cVid == vid {
				return c
			}
		}
	}
	return nil
}

// GetNearestPad finds the nearest charging pad to a position.
func (a *Airspace) GetNearestPad(ws *Workspace, pos VertexID) VertexID {
	posVertex := ws.Vertices[pos]
	nearestPad := VertexID(0)
	nearestDist := float64(1e9)

	for _, c := range a.Corridors {
		padVertex := ws.Vertices[c.BaseVertex]
		dx := posVertex.Pos.X - padVertex.Pos.X
		dy := posVertex.Pos.Y - padVertex.Pos.Y
		dz := posVertex.Pos.Z - padVertex.Pos.Z
		dist := dx*dx + dy*dy + dz*dz

		if dist < nearestDist {
			nearestDist = dist
			nearestPad = c.BaseVertex
		}
	}

	return nearestPad
}

// LayersAbove returns layers above the given layer.
func LayersAbove(l AirspaceLayer) []AirspaceLayer {
	var result []AirspaceLayer
	for _, layer := range AllLayers() {
		if layer > l {
			result = append(result, layer)
		}
	}
	return result
}

// LayersBelow returns layers below the given layer.
func LayersBelow(l AirspaceLayer) []AirspaceLayer {
	var result []AirspaceLayer
	for _, layer := range AllLayers() {
		if layer < l {
			result = append(result, layer)
		}
	}
	return result
}

// NextLayerUp returns the next layer up, or same layer if at top.
func NextLayerUp(l AirspaceLayer) AirspaceLayer {
	switch l {
	case LayerGround:
		return Layer1
	case Layer1:
		return Layer2
	case Layer2:
		return Layer3
	default:
		return l
	}
}

// NextLayerDown returns the next layer down, or same layer if at ground.
func NextLayerDown(l AirspaceLayer) AirspaceLayer {
	switch l {
	case Layer3:
		return Layer2
	case Layer2:
		return Layer1
	case Layer1:
		return LayerGround
	default:
		return l
	}
}
