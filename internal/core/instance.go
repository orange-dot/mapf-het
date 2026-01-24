package core

// Instance represents a MAPF-HET problem instance.
// I = (W, R, T, Θ, D)
type Instance struct {
	Workspace *Workspace
	Robots    []*Robot
	Tasks     []*Task
	Deadline  float64 // Hard deadline (seconds)
}

// NewInstance creates an empty instance.
func NewInstance() *Instance {
	return &Instance{
		Workspace: NewWorkspace(),
		Robots:    nil,
		Tasks:     nil,
		Deadline:  0,
	}
}

// Validate checks instance consistency.
func (inst *Instance) Validate() error {
	// TODO: Check task locations exist
	// TODO: Check robot starts exist
	// TODO: Check precedence graph is acyclic
	// TODO: Check each task has compatible robot
	return nil
}

// RobotByID finds robot by ID.
func (inst *Instance) RobotByID(id RobotID) *Robot {
	for _, r := range inst.Robots {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// TaskByID finds task by ID.
func (inst *Instance) TaskByID(id TaskID) *Task {
	for _, t := range inst.Tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}
