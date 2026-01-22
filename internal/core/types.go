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

// ============================================================================
// CAPABILITY BITMASK (EK-KOR2 Integration)
// ============================================================================

// Capability represents a bitmask matching EK-KOR2 ekk_capability_t.
// Bits 0-3: Runtime state (thermal, power, gateway, v2g)
// Bits 4-7: MAPF-HET task types (swap battery, swap module, aerial, diagnose)
// Bits 8-11: Application-defined
type Capability uint16

// Capability bits matching EK-KOR2 (bits 4-7 for MAPF-HET tasks)
const (
	CapSwapBattery Capability = 1 << 4 // Full pack swap (TypeB only)
	CapSwapModule  Capability = 1 << 5 // Single module (TypeA or B)
	CapAerialTask  Capability = 1 << 6 // Aerial inspection (TypeC only)
	CapDiagnose    Capability = 1 << 7 // Diagnostic scan (TypeA or B)
)

// Predefined capability sets for robot types
const (
	CapRobotTypeA Capability = CapSwapModule | CapDiagnose
	CapRobotTypeB Capability = CapSwapBattery | CapSwapModule | CapDiagnose
	CapRobotTypeC Capability = CapAerialTask
)

// TaskTypeToCapability maps MAPF-HET task types to required capabilities.
func TaskTypeToCapability(t TaskType) Capability {
	switch t {
	case SwapBattery:
		return CapSwapBattery
	case SwapModule:
		return CapSwapModule
	case Diagnose:
		return CapDiagnose
	case Clean:
		return CapSwapModule // Cleaning uses same capability as module swap
	case AerialInspect, AerialDelivery, AerialSurvey:
		return CapAerialTask
	default:
		return 0
	}
}

// RobotCapabilities returns the capability bitmask for a robot type.
func RobotCapabilities(r RobotType) Capability {
	switch r {
	case TypeA:
		return CapRobotTypeA
	case TypeB:
		return CapRobotTypeB
	case TypeC:
		return CapRobotTypeC
	default:
		return 0
	}
}

// CanPerformCapability checks if capabilities satisfy task requirements.
func CanPerformCapability(have, need Capability) bool {
	return (have & need) == need
}
