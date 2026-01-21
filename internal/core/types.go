// Package core defines domain models for MAPF-HET.
package core

// RobotType classifies robot capabilities.
type RobotType int

const (
	TypeA RobotType = iota // Mobile: holonomic, 0.5 m/s, 50kg
	TypeB                  // Rail-mounted: 1D, 2.0 m/s, 500kg
	TypeC                  // Aerial: 3D, 15.0 m/s, 2kg (drone)
)

func (t RobotType) String() string {
	return [...]string{"TypeA", "TypeB", "TypeC"}[t]
}

// TaskType classifies task requirements.
type TaskType int

const (
	SwapBattery    TaskType = iota // Full pack swap (TypeB only)
	SwapModule                     // Single module (TypeA or B)
	Diagnose                       // Diagnostic scan (TypeA or B)
	Clean                          // Cleaning (TypeA only)
	AerialInspect                  // Visual inspection from air (TypeC only)
	AerialDelivery                 // Carry small parts (TypeC only)
	AerialSurvey                   // Mapping/monitoring (TypeC only)
)

func (t TaskType) String() string {
	return [...]string{"SwapBattery", "SwapModule", "Diagnose", "Clean", "AerialInspect", "AerialDelivery", "AerialSurvey"}[t]
}

// CompatibleRobots returns which robot types can perform a task.
func CompatibleRobots(t TaskType) []RobotType {
	switch t {
	case SwapBattery:
		return []RobotType{TypeB}
	case SwapModule, Diagnose:
		return []RobotType{TypeA, TypeB}
	case Clean:
		return []RobotType{TypeA}
	case AerialInspect, AerialDelivery, AerialSurvey:
		return []RobotType{TypeC}
	default:
		return nil
	}
}

// CanPerform checks if robot type can perform task type.
func CanPerform(r RobotType, t TaskType) bool {
	for _, rt := range CompatibleRobots(t) {
		if rt == r {
			return true
		}
	}
	return false
}
