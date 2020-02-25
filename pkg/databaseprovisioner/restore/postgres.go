package restore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	_ "github.com/lib/pq" // Add postgres sql driver
	"github.com/sirupsen/logrus"
)

// pgRestoreBinary contains the path to the pg_restore executable used to perform the actual
// database restore
const pgRestoreBinary = "/usr/bin/pg_restore"

// pgRestoreJobs contains the number of jobs used while restoring data
const pgRestoreJobs = "3"

type postgresRestore struct {
	logger   *logrus.Entry
	opts     *PostgresRestoreOpts
	stopChan chan struct{}

	dumpRestoreSeconds *prometheus.SummaryVec
}

// PostgresRestoreOpts contains the options required to perform a postgres dump restore
type PostgresRestoreOpts struct {
	Name     string
	Hostname string
	Port     int64
	Database string
	Username string
	Password string
	DumpFile string
}

// NewPostgresRestore creates a new instance of the postgres restore functionality
func NewPostgresRestore(
	logger *logrus.Entry,
	opts *PostgresRestoreOpts,
	dumpRestoreSeconds *prometheus.SummaryVec,
) (Restore, error) {
	return &postgresRestore{
		logger:   logger,
		opts:     opts,
		stopChan: make(chan struct{}, 1),

		dumpRestoreSeconds: dumpRestoreSeconds,
	}, nil
}

func (p *postgresRestore) Start() error {
	// Create context with cancel hook, used to stop long-running command execution
	ctx, cancel := context.WithCancel(context.TODO())

	startTime := time.Now()
	defer func() {
		p.dumpRestoreSeconds.WithLabelValues(p.opts.Database).Observe(time.Since(startTime).Seconds())
	}()

	// Cancel context when the the stopChan closes
	go func() {
		<-p.stopChan
		cancel()
	}()

	err := p.wait(ctx)
	if err != nil {
		return err
	}

	return p.pgrestore(ctx)
}

func (p *postgresRestore) Stop() {
	close(p.stopChan)
}

func (p *postgresRestore) restoreArgs() []string {
	return []string{
		"-h",
		p.opts.Hostname,
		"-p",
		strconv.Itoa(int(p.opts.Port)),
		"-U",
		p.opts.Username,
		"--dbname",
		p.opts.Database,
		"--jobs",
		pgRestoreJobs,
		"--no-owner",
		"--role",
		p.opts.Username,
		"--no-acl",
		"--if-exists",
		"--clean",
		"--exit-on-error",
		"-Fc",
		p.opts.DumpFile,
	}
}

func (p *postgresRestore) wait(ctx context.Context) error {
	// Connection string used to connect to the database with the right credentials
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s connect_timeout=5 sslmode=disable",
		p.opts.Hostname,
		p.opts.Port,
		p.opts.Username,
		p.opts.Password,
		p.opts.Database,
	)

	var doneChan = make(chan struct{}, 1)

	go func() {
		for {
			db, err := sql.Open("postgres", connStr)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			result, err := db.Query("select 1")
			if err != nil {
				db.Close() // nolint:errcheck
				time.Sleep(time.Second)
				continue
			}

			result.Close() // nolint:errcheck
			db.Close()     // nolint:errcheck

			close(doneChan)
			return
		}
	}()

	select {
	case <-doneChan:
		return nil
	case <-ctx.Done():
		return nil
	case <-time.After(waitdeadline):
		return ErrDatabaseUnavailable
	}
}

func (p *postgresRestore) pgrestore(ctx context.Context) error {
	p.logger.Infof("running pg_restore on %s", p.opts.Name)

	cmd := exec.CommandContext(ctx, pgRestoreBinary, p.restoreArgs()...) // nolint: gas
	cmd.Env = []string{"PGPASSWORD=" + p.opts.Password}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == context.Canceled {
		return nil
	} else if err != nil {
		p.logger.WithError(err).Errorf("database restore for %s failed", p.opts.Name)
		return err
	}

	p.logger.Infof("pg_restore done on %s", p.opts.Name)

	return nil
}
