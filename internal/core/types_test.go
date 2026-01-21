package core

import "testing"

func TestCanPerform(t *testing.T) {
	tests := []struct {
		robot RobotType
		task  TaskType
		want  bool
	}{
		{TypeA, SwapModule, true},
		{TypeA, SwapBattery, false},
		{TypeA, Diagnose, true},
		{TypeA, Clean, true},
		{TypeB, SwapModule, true},
		{TypeB, SwapBattery, true},
		{TypeB, Diagnose, true},
		{TypeB, Clean, false},
	}

	for _, tt := range tests {
		got := CanPerform(tt.robot, tt.task)
		if got != tt.want {
			t.Errorf("CanPerform(%v, %v) = %v, want %v",
				tt.robot, tt.task, got, tt.want)
		}
	}
}

func TestCompatibleRobots(t *testing.T) {
	// SwapBattery only TypeB
	compat := CompatibleRobots(SwapBattery)
	if len(compat) != 1 || compat[0] != TypeB {
		t.Errorf("SwapBattery should only be compatible with TypeB")
	}

	// SwapModule both types
	compat = CompatibleRobots(SwapModule)
	if len(compat) != 2 {
		t.Errorf("SwapModule should be compatible with both types")
	}
}
