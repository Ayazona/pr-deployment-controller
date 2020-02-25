package build

import (
	"context"
	"sync"

	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/test-environment-manager/pkg/controller/databasetemplate"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	claimlock = new(sync.Mutex)
)

// databaseclaim is responsible for claiming a database for a build
type databaseclaim struct {
	br *buildReconciler
}

type claimeddatabase struct {
	Name     string
	Username string
	Password string
	Host     string
	Port     int64
}

func newDatabaseClaim(br *buildReconciler) (*databaseclaim, error) {
	return &databaseclaim{br: br}, nil
}

func (cd *databaseclaim) claim(ctx context.Context) (*claimeddatabase, error) {
	claimlock.Lock()
	defer claimlock.Unlock()

	// Noop if databaseTemplate is empty
	if cd.br.environment.Spec.DatabaseTemplate == nil || *cd.br.environment.Spec.DatabaseTemplate == "" {
		return nil, nil
	}

	var db *testenvironmentv1alpha1.Database
	var err error

	// Try to lookup existing database used by this build
	db, err = cd.existingDatabase(ctx)
	if err != nil {
		return nil, err
	}

	// Use database if exists
	if db != nil {
		return cd.dbToClaim(db), nil
	}

	// Claim new database
	db, err = cd.claimDatabase(ctx)
	if err != nil {
		return nil, err
	}

	return cd.dbToClaim(db), nil
}

func (cd *databaseclaim) dbToClaim(db *testenvironmentv1alpha1.Database) *claimeddatabase {
	return &claimeddatabase{
		Name:     db.Status.DatabaseName,
		Username: db.Status.Username,
		Password: db.Status.Password,
		Host:     db.Status.Host,
		Port:     db.Status.Port,
	}
}

func (cd *databaseclaim) existingDatabase(ctx context.Context) (*testenvironmentv1alpha1.Database, error) {
	// Create label selector used to filter database objects
	labelSelector, err := NewBuildDatabaseLabelSelector(*cd.br.environment.Spec.DatabaseTemplate, cd.br.build.Name)
	if err != nil {
		return nil, err
	}

	// Retrieve existing database objects
	databases := &testenvironmentv1alpha1.DatabaseList{}
	err = cd.br.r.List(
		ctx,
		&client.ListOptions{Namespace: cd.br.options.Namespace, LabelSelector: labelSelector},
		databases,
	)
	if err != nil {
		return nil, err
	}

	// Return first occurrence if found
	if len(databases.Items) > 0 {
		return &databases.Items[0], nil
	}

	return nil, nil
}

func (cd *databaseclaim) claimDatabase(ctx context.Context) (*testenvironmentv1alpha1.Database, error) {
	db, err := cd.readyDatabase(ctx)
	if err != nil {
		return nil, err
	}

	// Update database claim values
	db.Labels[LabelClaimedBuild] = cd.br.build.Name
	db.Status.BuildName = cd.br.build.Name
	db.Status.Phase = testenvironmentv1alpha1.DatabaseClaimed
	db.OwnerReferences = nil
	if err = controllerutil.SetControllerReference(cd.br.build, db, cd.br.r.scheme); err != nil {
		return nil, err
	}

	// Perform the update
	err = cd.br.r.Update(ctx, db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (cd *databaseclaim) readyDatabase(ctx context.Context) (*testenvironmentv1alpha1.Database, error) {
	// Create label selector used to filter database objects
	labelSelector, err := databasetemplate.NewDatabaseTemplateLabelSelector(*cd.br.environment.Spec.DatabaseTemplate)
	if err != nil {
		return nil, err
	}

	// Retrieve existing database objects
	databases := &testenvironmentv1alpha1.DatabaseList{}
	err = cd.br.r.List(
		ctx,
		&client.ListOptions{Namespace: cd.br.options.Namespace, LabelSelector: labelSelector},
		databases,
	)
	if err != nil {
		return nil, err
	}

	for _, database := range databases.Items {
		database := database
		if database.Status.Phase == testenvironmentv1alpha1.DatabaseReady && database.Status.BuildName == "" {
			return &database, nil
		}
	}

	return nil, ErrNoAvailableDatabases
}
