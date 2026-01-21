package core

// Assignment maps tasks to robots.
type Assignment map[TaskID]RobotID

// TimedVertex is a position at a specific time.
type TimedVertex struct {
	V VertexID
	T float64 // Time
}

// Path is a sequence of timed positions.
type Path []TimedVertex

// Schedule maps tasks to start times.
type Schedule map[TaskID]float64

// Solution represents a complete MAPF-HET solution.
type Solution struct {
	Assignment Assignment
	Paths      map[RobotID]Path
	Schedule   Schedule
	Makespan   float64
	Feasible   bool
}

// NewSolution creates an empty solution.
func NewSolution() *Solution {
	return &Solution{
		Assignment: make(Assignment),
		Paths:      make(map[RobotID]Path),
		Schedule:   make(Schedule),
		Makespan:   0,
		Feasible:   false,
	}
}

// ComputeMakespan calculates max completion time.
func (s *Solution) ComputeMakespan(inst *Instance) float64 {
	maxC := 0.0
	for tid, start := range s.Schedule {
		task := inst.TaskByID(tid)
		if task == nil {
			continue
		}
		completion := start + task.Duration
		if completion > maxC {
			maxC = completion
		}
	}
	s.Makespan = maxC
	return maxC
}

// MeetDeadline checks if solution meets deadline.
func (s *Solution) MeetDeadline(deadline float64) bool {
	return s.Makespan <= deadline
}
