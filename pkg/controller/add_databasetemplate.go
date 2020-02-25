package controller

import "github.com/kolonialno/test-environment-manager/pkg/controller/databasetemplate"

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, databasetemplate.Add)
}
