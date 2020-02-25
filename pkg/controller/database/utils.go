package database

import (
	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
)

// Labels generates the labels used to identify databases
func Labels(database *testenvironmentv1alpha1.Database) map[string]string {
	var labels = make(map[string]string)

	labels["app"] = "testenvironment-postgres"
	labels["database"] = database.Name

	return labels
}
