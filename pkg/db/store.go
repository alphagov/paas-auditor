package db

import (
	"context"
	"database/sql"
	"encoding/json"
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

type EventDB interface {
	Init() error
	StoreCfAuditEvents(events []cfclient.Event) error
	GetCfAuditEvents(filter RawEventFilter) ([]cfclient.Event, error)
	GetLatestCfEventTime() (*time.Time, error)
}

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
		eventMetadataJSON, err := json.Marshal(&event.Metadata)
		if err != nil {
			return err
		}

		stmt := fmt.Sprintf(`
			insert into %s (
				guid, created_at, event_type, actor, actor_type, actor_name, actor_username, actee, actee_type, actee_name, organization_guid, space_guid, metadata
			) values (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, '')::uuid, NULLIF($12, '')::uuid, $13
			) on conflict do nothing
		`, CfAuditEventsTable)
		_, err = tx.Exec(stmt, event.GUID, event.CreatedAt, event.Type, event.Actor, event.ActorType, event.ActorName, event.ActorUsername, event.Actee, event.ActeeType, event.ActeeName, event.OrganizationGUID, event.SpaceGUID, eventMetadataJSON)
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
			organization_guid AS text,
			space_guid AS text,
			metadata
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
		bytesOfMetadataJSON := []byte{}
		err = rows.Scan(
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
			&bytesOfMetadataJSON,
		)
		if err != nil {
			return nil, err
		}
		if len(bytesOfMetadataJSON) > 0 {
			err = json.Unmarshal(bytesOfMetadataJSON, &event.Metadata)
			if err != nil {
				return nil, err
			}
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *EventStore) GetLatestCfEventTime() (*time.Time, error) {
	ctx, cancel := context.WithTimeout(s.ctx, DefaultQueryTimeout)
	defer cancel()
	row := s.db.QueryRowContext(ctx, `
		select
			created_at
		from
			`+CfAuditEventsTable+`
		order by
			created_at DESC
		limit 1
	`)

	var createdAt time.Time
	err := row.Scan(&createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &createdAt, nil
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
	return filepath.Join(root, "pkg", "db", "sql")
}

func schemaFile(filename string) string {
	return filepath.Join(schemaDir(), filename)
}
