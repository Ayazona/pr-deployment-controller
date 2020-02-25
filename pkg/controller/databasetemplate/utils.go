package databasetemplate

import (
	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
)

// CountUnclaimedDatabases counts the amount of unclaimed databases
func CountUnclaimedDatabases(databases *testenvironmentv1alpha1.DatabaseList) int64 {
	var unclaimedDatabases int64

	for _, database := range databases.Items {
		if database.Status.Phase != testenvironmentv1alpha1.DatabaseClaimed {
			unclaimedDatabases++
		}
	}

	return unclaimedDatabases
}
