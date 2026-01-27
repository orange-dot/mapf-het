// Package main provides instance generation for MAPF-HET benchmarks.
// Generates deterministic test instances with configurable parameters.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// InstanceParams defines parameters for instance generation.
type InstanceParams struct {
	Seed            int64   `json:"seed"`
	NumAgents       int     `json:"num_agents"`
	GridWidth       int     `json:"grid_width"`
	GridHeight      int     `json:"grid_height"`
	TaskCount       int     `json:"task_count"`
	DeadlineMin     float64 `json:"deadline_min"`
	DeadlineMax     float64 `json:"deadline_max"`
	ChargingDensity float64 `json:"charging_density"` // Fraction of vertices that are charging pads
	DroneRatio      float64 `json:"drone_ratio"`      // Fraction of agents that are drones
	RailRatio       float64 `json:"rail_ratio"`       // Fraction of agents that are rail-mounted
	AirspaceLayers  int     `json:"airspace_layers"`  // Number of altitude layers (0 = ground only)
}

// Vertex represents a location in the workspace.
type Vertex struct {
	ID         int       `json:"id"`
	X          float64   `json:"x"`
	Y          float64   `json:"y"`
	Z          float64   `json:"z"`
	Layer      int       `json:"layer"`       // Airspace layer
	IsPad      bool      `json:"is_pad"`      // Charging pad
	IsCorridor bool      `json:"is_corridor"` // Vertical corridor
	NoFlyZone  bool      `json:"no_fly_zone"`
	Restrict   []string  `json:"restrict,omitempty"` // Allowed robot types
}

// Edge connects two vertices.
type Edge struct {
	From   int     `json:"from"`
	To     int     `json:"to"`
	Length float64 `json:"length"`
}

// Robot represents an agent.
type Robot struct {
	ID              int     `json:"id"`
	Type            string  `json:"type"` // "TypeA", "TypeB", "TypeC"
	Start           int     `json:"start"`
	HomeBase        int     `json:"home_base,omitempty"`
	BatteryCapacity float64 `json:"battery_capacity,omitempty"`
}

// Task represents work to be performed.
type Task struct {
	ID         int     `json:"id"`
	Type       string  `json:"type"`
	Location   int     `json:"location"`
	Duration   float64 `json:"duration"`
	Deadline   float64 `json:"deadline,omitempty"` // Per-task deadline (0 = use global)
	Precedence []int   `json:"precedence,omitempty"`
}

// Instance represents a complete MAPF-HET problem.
type Instance struct {
	Name       string         `json:"name"`
	Params     InstanceParams `json:"params"`
	Vertices   []Vertex       `json:"vertices"`
	Edges      []Edge         `json:"edges"`
	Robots     []Robot        `json:"robots"`
	Tasks      []Task         `json:"tasks"`
	Deadline   float64        `json:"deadline"` // Global deadline
	Generated  string         `json:"generated"`
}

// TaskTypes and their compatible robot types
var taskTypes = map[string][]string{
	"SwapBattery":    {"TypeB"},
	"SwapModule":     {"TypeA", "TypeB"},
	"Diagnose":       {"TypeA", "TypeB"},
	"Clean":          {"TypeA"},
	"AerialInspect":  {"TypeC"},
	"AerialDelivery": {"TypeC"},
	"AerialSurvey":   {"TypeC"},
}

// Nominal task durations (mean, stddev)
var taskDurations = map[string][2]float64{
	"SwapBattery":    {120.0, 15.0},
	"SwapModule":     {45.0, 8.0},
	"Diagnose":       {30.0, 5.0},
	"Clean":          {60.0, 10.0},
	"AerialInspect":  {20.0, 3.0},
	"AerialDelivery": {15.0, 2.0},
	"AerialSurvey":   {90.0, 12.0},
}

// generateInstance creates a MAPF-HET instance from parameters.
func generateInstance(params InstanceParams) *Instance {
	rng := rand.New(rand.NewSource(params.Seed))

	inst := &Instance{
		Name:      fmt.Sprintf("mapfhet_%d_%dx%d_%d", params.NumAgents, params.GridWidth, params.GridHeight, params.Seed),
		Params:    params,
		Generated: time.Now().UTC().Format(time.RFC3339),
	}

	// Generate ground-level grid vertices
	numGroundVertices := params.GridWidth * params.GridHeight
	for y := 0; y < params.GridHeight; y++ {
		for x := 0; x < params.GridWidth; x++ {
			id := y*params.GridWidth + x
			v := Vertex{
				ID:    id,
				X:     float64(x),
				Y:     float64(y),
				Z:     0,
				Layer: 0,
			}

			// Charging pads at specified density
			if rng.Float64() < params.ChargingDensity {
				v.IsPad = true
			}

			// Corridors at edges and random locations
			if x == 0 || x == params.GridWidth-1 || y == 0 || y == params.GridHeight-1 {
				v.IsCorridor = true
			} else if rng.Float64() < 0.1 {
				v.IsCorridor = true
			}

			inst.Vertices = append(inst.Vertices, v)
		}
	}

	// Generate aerial layers if requested
	if params.AirspaceLayers > 0 {
		layerHeights := []float64{5.0, 10.0, 15.0}
		for layer := 1; layer <= params.AirspaceLayers && layer <= 3; layer++ {
			for y := 0; y < params.GridHeight; y++ {
				for x := 0; x < params.GridWidth; x++ {
					groundID := y*params.GridWidth + x
					id := numGroundVertices + (layer-1)*numGroundVertices + groundID

					v := Vertex{
						ID:    id,
						X:     float64(x),
						Y:     float64(y),
						Z:     layerHeights[layer-1],
						Layer: layer,
					}

					// Random no-fly zones (10% of aerial vertices)
					if rng.Float64() < 0.1 {
						v.NoFlyZone = true
					}

					inst.Vertices = append(inst.Vertices, v)
				}
			}
		}
	}

	// Generate ground edges (4-connected grid)
	for y := 0; y < params.GridHeight; y++ {
		for x := 0; x < params.GridWidth; x++ {
			id := y*params.GridWidth + x
			// Right neighbor
			if x < params.GridWidth-1 {
				inst.Edges = append(inst.Edges, Edge{From: id, To: id + 1, Length: 1.0})
				inst.Edges = append(inst.Edges, Edge{From: id + 1, To: id, Length: 1.0})
			}
			// Down neighbor
			if y < params.GridHeight-1 {
				inst.Edges = append(inst.Edges, Edge{From: id, To: id + params.GridWidth, Length: 1.0})
				inst.Edges = append(inst.Edges, Edge{From: id + params.GridWidth, To: id, Length: 1.0})
			}
		}
	}

	// Generate aerial edges (8-connected grid within layer + vertical corridors)
	if params.AirspaceLayers > 0 {
		for layer := 1; layer <= params.AirspaceLayers && layer <= 3; layer++ {
			layerOffset := numGroundVertices + (layer-1)*numGroundVertices
			for y := 0; y < params.GridHeight; y++ {
				for x := 0; x < params.GridWidth; x++ {
					id := layerOffset + y*params.GridWidth + x

					// 8-connected within layer
					for dy := -1; dy <= 1; dy++ {
						for dx := -1; dx <= 1; dx++ {
							if dx == 0 && dy == 0 {
								continue
							}
							nx, ny := x+dx, y+dy
							if nx >= 0 && nx < params.GridWidth && ny >= 0 && ny < params.GridHeight {
								nid := layerOffset + ny*params.GridWidth + nx
								dist := math.Sqrt(float64(dx*dx + dy*dy))
								inst.Edges = append(inst.Edges, Edge{From: id, To: nid, Length: dist})
							}
						}
					}

					// Vertical corridor connections
					groundID := y*params.GridWidth + x
					if inst.Vertices[groundID].IsCorridor {
						// Connect to layer below
						if layer == 1 {
							inst.Edges = append(inst.Edges, Edge{From: id, To: groundID, Length: 5.0})
							inst.Edges = append(inst.Edges, Edge{From: groundID, To: id, Length: 5.0})
						} else {
							belowID := numGroundVertices + (layer-2)*numGroundVertices + groundID
							inst.Edges = append(inst.Edges, Edge{From: id, To: belowID, Length: 5.0})
							inst.Edges = append(inst.Edges, Edge{From: belowID, To: id, Length: 5.0})
						}
					}
				}
			}
		}
	}

	// Generate robots
	numDrones := int(float64(params.NumAgents) * params.DroneRatio)
	numRail := int(float64(params.NumAgents) * params.RailRatio)
	numMobile := params.NumAgents - numDrones - numRail

	// Collect charging pads for drone home bases
	var chargingPads []int
	for _, v := range inst.Vertices {
		if v.IsPad && v.Layer == 0 {
			chargingPads = append(chargingPads, v.ID)
		}
	}

	usedStarts := make(map[int]bool)
	robotID := 0

	// Mobile robots (TypeA)
	for i := 0; i < numMobile; i++ {
		start := rng.Intn(numGroundVertices)
		for usedStarts[start] {
			start = rng.Intn(numGroundVertices)
		}
		usedStarts[start] = true

		inst.Robots = append(inst.Robots, Robot{
			ID:    robotID,
			Type:  "TypeA",
			Start: start,
		})
		robotID++
	}

	// Rail robots (TypeB) - start on edges
	for i := 0; i < numRail; i++ {
		var start int
		if rng.Float64() < 0.5 {
			// Horizontal edge
			y := 0
			if rng.Float64() < 0.5 {
				y = params.GridHeight - 1
			}
			start = y*params.GridWidth + rng.Intn(params.GridWidth)
		} else {
			// Vertical edge
			x := 0
			if rng.Float64() < 0.5 {
				x = params.GridWidth - 1
			}
			start = rng.Intn(params.GridHeight)*params.GridWidth + x
		}
		for usedStarts[start] {
			start = rng.Intn(numGroundVertices)
		}
		usedStarts[start] = true

		inst.Robots = append(inst.Robots, Robot{
			ID:    robotID,
			Type:  "TypeB",
			Start: start,
		})
		robotID++
	}

	// Drones (TypeC)
	for i := 0; i < numDrones; i++ {
		var homeBase int
		if len(chargingPads) > 0 {
			homeBase = chargingPads[rng.Intn(len(chargingPads))]
		} else {
			homeBase = rng.Intn(numGroundVertices)
		}

		start := homeBase
		for usedStarts[start] {
			start = rng.Intn(numGroundVertices)
		}
		usedStarts[start] = true

		inst.Robots = append(inst.Robots, Robot{
			ID:              robotID,
			Type:            "TypeC",
			Start:           start,
			HomeBase:        homeBase,
			BatteryCapacity: 100.0, // 100 Wh
		})
		robotID++
	}

	// Generate tasks
	// Distribute task types based on available robot types
	availableTypes := make(map[string]bool)
	for _, r := range inst.Robots {
		availableTypes[r.Type] = true
	}

	var possibleTasks []string
	for taskType, compatibleRobots := range taskTypes {
		for _, rt := range compatibleRobots {
			if availableTypes[rt] {
				possibleTasks = append(possibleTasks, taskType)
				break
			}
		}
	}

	usedLocations := make(map[int]bool)
	for i := 0; i < params.TaskCount; i++ {
		taskType := possibleTasks[rng.Intn(len(possibleTasks))]

		// Choose location based on task type
		var location int
		if taskType == "AerialInspect" || taskType == "AerialDelivery" || taskType == "AerialSurvey" {
			// Aerial tasks can be at any altitude
			if params.AirspaceLayers > 0 && rng.Float64() < 0.7 {
				layer := 1 + rng.Intn(params.AirspaceLayers)
				layerOffset := numGroundVertices + (layer-1)*numGroundVertices
				location = layerOffset + rng.Intn(numGroundVertices)
			} else {
				location = rng.Intn(numGroundVertices)
			}
		} else {
			// Ground tasks
			location = rng.Intn(numGroundVertices)
		}

		// Avoid duplicate locations for simplicity
		attempts := 0
		for usedLocations[location] && attempts < 100 {
			if taskType == "AerialInspect" || taskType == "AerialDelivery" || taskType == "AerialSurvey" {
				if params.AirspaceLayers > 0 && rng.Float64() < 0.7 {
					layer := 1 + rng.Intn(params.AirspaceLayers)
					layerOffset := numGroundVertices + (layer-1)*numGroundVertices
					location = layerOffset + rng.Intn(numGroundVertices)
				} else {
					location = rng.Intn(numGroundVertices)
				}
			} else {
				location = rng.Intn(numGroundVertices)
			}
			attempts++
		}
		usedLocations[location] = true

		// Duration with some randomness
		dur := taskDurations[taskType]
		duration := dur[0] + rng.NormFloat64()*dur[1]
		if duration < dur[0]*0.5 {
			duration = dur[0] * 0.5
		}

		// Per-task deadline (random within range)
		deadline := params.DeadlineMin + rng.Float64()*(params.DeadlineMax-params.DeadlineMin)

		inst.Tasks = append(inst.Tasks, Task{
			ID:       i,
			Type:     taskType,
			Location: location,
			Duration: duration,
			Deadline: deadline,
		})
	}

	// Global deadline is max of per-task deadlines
	maxDeadline := params.DeadlineMax
	for _, t := range inst.Tasks {
		if t.Deadline > maxDeadline {
			maxDeadline = t.Deadline
		}
	}
	inst.Deadline = maxDeadline * 1.2 // 20% buffer

	return inst
}

func main() {
	// Parse flags
	seed := flag.Int64("seed", 42, "Random seed for deterministic generation")
	numAgents := flag.Int("agents", 10, "Number of agents")
	gridWidth := flag.Int("width", 10, "Grid width")
	gridHeight := flag.Int("height", 10, "Grid height")
	taskCount := flag.Int("tasks", 20, "Number of tasks")
	deadlineMin := flag.Float64("deadline-min", 300, "Minimum task deadline (seconds)")
	deadlineMax := flag.Float64("deadline-max", 600, "Maximum task deadline (seconds)")
	chargingDensity := flag.Float64("charging", 0.1, "Charging pad density (0-1)")
	droneRatio := flag.Float64("drones", 0.2, "Fraction of agents that are drones")
	railRatio := flag.Float64("rail", 0.1, "Fraction of agents that are rail-mounted")
	airspaceLayers := flag.Int("layers", 2, "Number of airspace layers (0 = ground only)")
	outputDir := flag.String("output", "testdata", "Output directory")
	scalingMode := flag.Bool("scaling", false, "Generate scaling test instances (10, 50, 100, 500, 1000, 2048 agents)")

	flag.Parse()

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	var instances []*Instance

	if *scalingMode {
		// Generate scaling test suite
		scalingSizes := []int{10, 50, 100, 500, 1000, 2048}
		for _, size := range scalingSizes {
			// Grid size scales with sqrt of agents
			gridSize := int(math.Ceil(math.Sqrt(float64(size)) * 3))
			if gridSize < 10 {
				gridSize = 10
			}

			params := InstanceParams{
				Seed:            *seed,
				NumAgents:       size,
				GridWidth:       gridSize,
				GridHeight:      gridSize,
				TaskCount:       size * 2, // 2 tasks per agent
				DeadlineMin:     *deadlineMin,
				DeadlineMax:     *deadlineMax,
				ChargingDensity: *chargingDensity,
				DroneRatio:      *droneRatio,
				RailRatio:       *railRatio,
				AirspaceLayers:  *airspaceLayers,
			}

			inst := generateInstance(params)
			instances = append(instances, inst)
		}
	} else {
		// Generate single instance
		params := InstanceParams{
			Seed:            *seed,
			NumAgents:       *numAgents,
			GridWidth:       *gridWidth,
			GridHeight:      *gridHeight,
			TaskCount:       *taskCount,
			DeadlineMin:     *deadlineMin,
			DeadlineMax:     *deadlineMax,
			ChargingDensity: *chargingDensity,
			DroneRatio:      *droneRatio,
			RailRatio:       *railRatio,
			AirspaceLayers:  *airspaceLayers,
		}

		inst := generateInstance(params)
		instances = append(instances, inst)
	}

	// Write instances to files
	for _, inst := range instances {
		filename := filepath.Join(*outputDir, inst.Name+".json")
		data, err := json.MarshalIndent(inst, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling instance %s: %v\n", inst.Name, err)
			continue
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing instance %s: %v\n", filename, err)
			continue
		}

		fmt.Printf("Generated: %s (%d agents, %d tasks, %dx%d grid)\n",
			filename, inst.Params.NumAgents, inst.Params.TaskCount,
			inst.Params.GridWidth, inst.Params.GridHeight)
	}
}
