package controller

import "github.com/kolonialno/pr-deployment-controller/pkg/controller/database"

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, database.Add)
}
