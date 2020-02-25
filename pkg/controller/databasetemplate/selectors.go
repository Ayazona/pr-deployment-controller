package databasetemplate

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// NewDatabaseTemplateLabelSelector creates a label selector for databases based on a databasetemplate name
func NewDatabaseTemplateLabelSelector(databasetemplateName string) (labels.Selector, error) {
	var selector = labels.NewSelector()

	requirement, err := labels.NewRequirement(
		LabelDatabaseTemplate,
		selection.Equals,
		[]string{databasetemplateName},
	)
	if err != nil {
		return nil, err
	}

	selector = selector.Add(*requirement)

	return selector, nil
}
