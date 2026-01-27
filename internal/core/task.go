package core

// TaskID is a unique task identifier.
type TaskID int

// Task represents work to be performed.
type Task struct {
	ID          TaskID
	Type        TaskType
	Location    VertexID  // Where task must be performed
	Duration    float64   // Nominal duration (seconds)
	DurationStd float64   // Std dev for stochastic sim
	Deadline    float64   // Per-task deadline (seconds); 0 = use global deadline
	Precedence  []TaskID  // Must complete before this task
}

// HasDeadline returns true if task has a specific per-task deadline.
func (t *Task) HasDeadline() bool {
	return t.Deadline > 0
}

// GetDeadline returns the task's deadline, or the global deadline if not set.
func (t *Task) GetDeadline(globalDeadline float64) float64 {
	if t.Deadline > 0 {
		return t.Deadline
	}
	return globalDeadline
}

// NominalDuration returns default (mean, std) for task type.
func NominalDuration(t TaskType) (mean, std float64) {
	switch t {
	case SwapBattery:
		return 120.0, 15.0
	case SwapModule:
		return 45.0, 8.0
	case Diagnose:
		return 30.0, 5.0
	case Clean:
		return 60.0, 10.0
	case AerialInspect:
		return 20.0, 3.0 // Quick aerial inspection
	case AerialDelivery:
		return 15.0, 2.0 // Fast delivery
	case AerialSurvey:
		return 90.0, 12.0 // Longer survey mission
	default:
		return 0, 0
	}
}

// NewTask creates a task with nominal duration.
func NewTask(id TaskID, typ TaskType, loc VertexID) *Task {
	mean, std := NominalDuration(typ)
	return &Task{
		ID:          id,
		Type:        typ,
		Location:    loc,
		Duration:    mean,
		DurationStd: std,
		Deadline:    0, // Use global deadline
		Precedence:  nil,
	}
}

// NewTaskWithDeadline creates a task with a specific per-task deadline.
func NewTaskWithDeadline(id TaskID, typ TaskType, loc VertexID, deadline float64) *Task {
	task := NewTask(id, typ, loc)
	task.Deadline = deadline
	return task
}
