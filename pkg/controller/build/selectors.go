package build

import (
	"github.com/kolonialno/test-environment-manager/pkg/controller/databasetemplate"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// NewBuildDatabaseLabelSelector creates a label selector for databases based on a databasetemplate name and build name
func NewBuildDatabaseLabelSelector(databasetemplateName, buildName string) (labels.Selector, error) {
	var selector = labels.NewSelector()

	templaterequirement, err := labels.NewRequirement(
		databasetemplate.LabelDatabaseTemplate,
		selection.Equals,
		[]string{databasetemplateName},
	)
	if err != nil {
		return nil, err
	}

	selector = selector.Add(*templaterequirement)

	buildrequirement, err := labels.NewRequirement(
		LabelClaimedBuild,
		selection.Equals,
		[]string{buildName},
	)
	if err != nil {
		return nil, err
	}

	selector = selector.Add(*buildrequirement)

	return selector, nil
}
