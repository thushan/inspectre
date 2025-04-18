package repository

import (
	"github.com/thushan/inspectre/internal/core/types"
)

// Use the common Task type from types package
type Task = types.Task

// Use the common Repository type from types package
type Repository = types.Repository

// Use the common Auth type from types package
type Auth = types.Auth

// Use the common Config type from types package
type Config = types.Config

// RepositoryManager handles repository operations
type RepositoryManager interface {
	types.RepositoryManager
}
