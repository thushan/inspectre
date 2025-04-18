package task

import (
	"github.com/thushan/inspectre/internal/core/types"
)

// Use the common Task type from types package
type Task = types.Task

// TaskManager handles task operations
type TaskManager interface {
	types.TaskManager
}
