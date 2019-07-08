package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/lib/pq"
)

const (
	CfAuditEventsTable  = "cf_audit_events"
	DefaultInitTimeout  = 15 * time.Minute
	DefaultStoreTimeout = 10 * time.Minute
	DefaultQueryTimeout = 60 * time.Second
)

type EventStore struct {
	db     *sql.DB
	logger lager.Logger
	ctx    context.Context
}

func NewEventStore(ctx context.Context, db *sql.DB, logger lager.Logger) *EventStore {
	return &EventStore{
		db:     db,
		logger: logger.Session("event-store"),
		ctx:    ctx,
	}
}

// Init initialises the database tables and functions
func (s *EventStore) Init() error {
	s.logger.Info("initializing")
	ctx, cancel := context.WithTimeout(s.ctx, DefaultInitTimeout)
	defer cancel()

	if err := s.runSQLFilesInTransaction(
		ctx,
		"create_cf_audit_events.sql",
	); err != nil {
		return err
	}

	s.logger.Info("initialized")
	return nil
}

func (s *EventStore) StoreCfAuditEvents(events []cfclient.Event) error {
	ctx, cancel := context.WithTimeout(s.ctx, DefaultStoreTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, event := range events {
		var err error
		if event.OrganizationGUID != "" && event.SpaceGUID != "" {
			stmt := fmt.Sprintf(`
				insert into %s (
					guid, created_at, event_type, actor, actor_type, actor_name, actor_username, actee, actee_type, actee_name, organization_guid, space_guid
				) values (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
				) on conflict do nothing
			`, CfAuditEventsTable)
			_, err = tx.Exec(stmt, event.GUID, event.CreatedAt, event.Type, event.Actor, event.ActorType, event.ActorName, event.ActorUsername, event.Actee, event.ActeeType, event.ActeeName, event.OrganizationGUID, event.SpaceGUID)
		} else if event.OrganizationGUID != "" {
			stmt := fmt.Sprintf(`
				insert into %s (
					guid, created_at, event_type, actor, actor_type, actor_name, actor_username, actee, actee_type, actee_name, organization_guid, space_guid
				) values (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, null::uuid
				) on conflict do nothing
			`, CfAuditEventsTable)
			_, err = tx.Exec(stmt, event.GUID, event.CreatedAt, event.Type, event.Actor, event.ActorType, event.ActorName, event.ActorUsername, event.Actee, event.ActeeType, event.ActeeName, event.OrganizationGUID)
		} else if event.SpaceGUID != "" {
			stmt := fmt.Sprintf(`
				insert into %s (
					guid, created_at, event_type, actor, actor_type, actor_name, actor_username, actee, actee_type, actee_name, organization_guid, space_guid
				) values (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, null::uuid, $11
				) on conflict do nothing
			`, CfAuditEventsTable)
			_, err = tx.Exec(stmt, event.GUID, event.CreatedAt, event.Type, event.Actor, event.ActorType, event.ActorName, event.ActorUsername, event.Actee, event.ActeeType, event.ActeeName, event.SpaceGUID)
		} else {
			stmt := fmt.Sprintf(`
				insert into %s (
					guid, created_at, event_type, actor, actor_type, actor_name, actor_username, actee, actee_type, actee_name, organization_guid, space_guid
				) values (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, null::uuid, null::uuid
				) on conflict do nothing
			`, CfAuditEventsTable)
			_, err = tx.Exec(stmt, event.GUID, event.CreatedAt, event.Type, event.Actor, event.ActorType, event.ActorName, event.ActorUsername, event.Actee, event.ActeeType, event.ActeeName)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

type RawEventFilter struct {
	Reverse bool
	Limit   int
	Kind    string
}

func (s *EventStore) GetCfAuditEvents(filter RawEventFilter) ([]cfclient.Event, error) {
	events := []cfclient.Event{}
	sortDirection := "desc"
	if filter.Reverse {
		sortDirection = "asc"
	}
	limit := ""
	if filter.Limit > 0 {
		limit = fmt.Sprintf(`limit %d`, filter.Limit)
	}
	ctx, cancel := context.WithTimeout(s.ctx, DefaultQueryTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`
		select
			guid,
			created_at,
			event_type,
			actor,
			actor_type,
			actor_name,
			actor_username,
			actee,
			actee_type,
			actee_name,
			organization_guid,
			space_guid
		from
			` + CfAuditEventsTable + `
		order by
			id ` + sortDirection + `
		` + limit + `
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		event := cfclient.Event{}
		err := rows.Scan(
			&event.GUID,
			&event.CreatedAt,
			&event.Type,
			&event.Actor,
			&event.ActorType,
			&event.ActorName,
			&event.ActorUsername,
			&event.Actee,
			&event.ActeeType,
			&event.ActeeName,
			&event.OrganizationGUID,
			&event.SpaceGUID,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *EventStore) runSQLFilesInTransaction(ctx context.Context, filenames ...string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, filename := range filenames {
		if err := s.runSQLFile(tx, filename); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *EventStore) runSQLFile(tx *sql.Tx, filename string) error {
	startTime := time.Now()
	s.logger.Info("run-sql-file", map[string]interface{}{"sqlFile": filename})

	defer func() {
		s.logger.Info("finish-sql-file", lager.Data{
			"sqlFile": filename,
			"elapsed": time.Since(startTime),
		})
	}()

	schemaFilename := schemaFile(filename)
	sql, err := ioutil.ReadFile(schemaFilename)
	if err != nil {
		return fmt.Errorf("failed to execute sql file %s: %s", schemaFilename, err)
	}

	_, err = tx.Exec(string(sql))
	if err != nil {
		return wrapPqError(err, schemaFilename)
	}

	return nil
}

// queryJSON returns rows as a json blobs, which makes it easier to decode into structs.
func queryJSON(tx *sql.Tx, q string, args ...interface{}) (*sql.Rows, error) {
	return tx.Query(fmt.Sprintf(`
		with q as ( %s )
		select row_to_json(q.*) from q;
	`, q), args...)
}

func wrapPqError(err error, prefix string) error {
	msg := err.Error()
	if err, ok := err.(*pq.Error); ok {
		msg = err.Message
		if err.Detail != "" {
			msg += ": " + err.Detail
		}
		if err.Hint != "" {
			msg += ": " + err.Hint
		}
		if err.Where != "" {
			msg += ": " + err.Where
		}
	}
	return fmt.Errorf("%s: %s", prefix, msg)
}

func schemaDir() string {
	root := os.Getenv("APP_ROOT")
	if root == "" {
		root = os.Getenv("PWD")
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	return filepath.Join(root, "db", "sql")
}

func schemaFile(filename string) string {
	return filepath.Join(schemaDir(), filename)
}
