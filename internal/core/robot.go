package core

// RobotID is a unique robot identifier.
type RobotID int

// MoveAction represents drone movement types for energy calculation.
type MoveAction int

const (
	ActionHover          MoveAction = iota // Stationary hover
	ActionMoveHorizontal                   // Horizontal movement in layer
	ActionClimb                            // Ascending to higher layer
	ActionDescend                          // Descending to lower layer
)

// Robot represents an agent in the system.
type Robot struct {
	ID    RobotID
	Type  RobotType
	Start VertexID // Initial position
	// Drone-specific fields (TypeC only)
	BatteryCapacity float64  // Wh (e.g., 100 Wh for drone)
	CurrentBattery  float64  // Current charge in Wh
	HomeBase        VertexID // Charging pad location
}

// Speed returns robot's max velocity (m/s).
func (r *Robot) Speed() float64 {
	switch r.Type {
	case TypeA:
		return 0.5
	case TypeB:
		return 2.0
	case TypeC:
		return 15.0 // Drones are fast
	default:
		return 0.0
	}
}

// Payload returns max payload (kg).
func (r *Robot) Payload() float64 {
	switch r.Type {
	case TypeA:
		return 50.0
	case TypeB:
		return 500.0
	case TypeC:
		return 2.0 // Drones carry small payloads
	default:
		return 0.0
	}
}

// IsDrone returns true if this robot is an aerial drone.
func (r *Robot) IsDrone() bool {
	return r.Type == TypeC
}

// EnergyConsumption returns power consumption in Watts for a movement action.
// Only applies to TypeC drones; returns 0 for ground robots.
func (r *Robot) EnergyConsumption(action MoveAction) float64 {
	if r.Type != TypeC {
		return 0 // Ground robots don't track energy
	}

	baseHover := 50.0 // Watts for hovering
	switch action {
	case ActionHover:
		return baseHover * 1.0
	case ActionMoveHorizontal:
		return baseHover * 1.5
	case ActionClimb:
		return baseHover * 2.5 // Climbing is expensive
	case ActionDescend:
		return baseHover * 0.8 // Descending is cheaper
	default:
		return baseHover
	}
}

// EnergyForDistance calculates energy needed to travel a distance.
// Returns energy in Wh.
func (r *Robot) EnergyForDistance(dist float64, action MoveAction) float64 {
	if r.Type != TypeC {
		return 0
	}
	travelTime := dist / r.Speed() // seconds
	power := r.EnergyConsumption(action)
	return power * travelTime / 3600.0 // Convert Ws to Wh
}

// EnergyForLayerChange returns energy needed to change altitude layers.
// Returns energy in Wh.
func (r *Robot) EnergyForLayerChange(from, to AirspaceLayer) float64 {
	if r.Type != TypeC {
		return 0
	}

	heightDiff := to.Height() - from.Height()
	if heightDiff == 0 {
		return 0
	}

	climbSpeed := 2.0 // m/s vertical
	var action MoveAction
	if heightDiff > 0 {
		action = ActionClimb
	} else {
		action = ActionDescend
		heightDiff = -heightDiff
	}

	travelTime := heightDiff / climbSpeed
	power := r.EnergyConsumption(action)
	return power * travelTime / 3600.0
}

// CanReachWithEnergy checks if drone can reach destination with current battery.
// Takes into account distance, layer changes, and a safety margin.
func (r *Robot) CanReachWithEnergy(dist float64, layerChanges int) bool {
	if r.Type != TypeC {
		return true // Ground robots don't have energy constraints
	}

	// Estimate travel energy
	travelTime := dist / r.Speed()
	avgPower := 75.0 // Average between hover and move
	travelEnergy := avgPower * travelTime / 3600.0

	// Layer change cost (5m per layer, 20Wh approx per change)
	layerEnergy := float64(layerChanges) * 0.02 // ~20Wh per layer change

	totalEnergy := travelEnergy + layerEnergy
	safetyMargin := 1.2 // 20% safety margin

	return r.CurrentBattery >= totalEnergy*safetyMargin
}

// BatteryPercentage returns current battery level as percentage.
func (r *Robot) BatteryPercentage() float64 {
	if r.Type != TypeC || r.BatteryCapacity <= 0 {
		return 100.0 // Ground robots always "full"
	}
	return (r.CurrentBattery / r.BatteryCapacity) * 100.0
}

// IsLowBattery returns true if battery is below critical threshold.
func (r *Robot) IsLowBattery() bool {
	if r.Type != TypeC {
		return false
	}
	return r.BatteryPercentage() < 20.0
}

// Recharge sets battery to full capacity.
func (r *Robot) Recharge() {
	if r.Type == TypeC {
		r.CurrentBattery = r.BatteryCapacity
	}
}

// ConsumeEnergy reduces battery by specified amount.
func (r *Robot) ConsumeEnergy(wh float64) {
	if r.Type == TypeC {
		r.CurrentBattery -= wh
		if r.CurrentBattery < 0 {
			r.CurrentBattery = 0
		}
	}
}

// NewDrone creates a TypeC robot with battery parameters.
func NewDrone(id RobotID, start VertexID, homeBase VertexID, batteryWh float64) *Robot {
	return &Robot{
		ID:              id,
		Type:            TypeC,
		Start:           start,
		BatteryCapacity: batteryWh,
		CurrentBattery:  batteryWh, // Start fully charged
		HomeBase:        homeBase,
	}
}
