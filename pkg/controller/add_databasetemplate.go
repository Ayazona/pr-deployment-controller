package controller

import "github.com/kolonialno/pr-deployment-controller/pkg/controller/databasetemplate"

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, databasetemplate.Add)
}
