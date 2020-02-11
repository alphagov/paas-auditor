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
	CFAuditEventsTable  = "cf_audit_events"
	ShipperCursorsTable = "shipper_cursors"

	DefaultInitTimeout  = 15 * time.Minute
	DefaultStoreTimeout = 10 * time.Minute
	DefaultQueryTimeout = 60 * time.Second
)

type EventDB interface {
	Init() error

	StoreCFAuditEvents(events []cfclient.Event) error
	GetCFAuditEvents(filter RawEventFilter) ([]cfclient.Event, error)
	GetLatestCFEventTime() (time.Time, error)
	GetCFEventCount() (int64, error)

	GetUnshippedCFAuditEventsForShipper(shipperName string) ([]cfclient.Event, error)
	UpdateShipperCursor(shipperName string, shipperTime string, shippedID string) error
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

	for _, filename := range []string{
		"create_cf_audit_events.sql",
		"create_shipper_cursors.sql",
	} {
		if err := s.runSQLFilesInTransaction(ctx, filename); err != nil {
			return err
		}
	}

	s.logger.Info("initialized")
	return nil
}

func (s *EventStore) StoreCFAuditEvents(events []cfclient.Event) error {
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
		`, CFAuditEventsTable)
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

func (s *EventStore) GetCFAuditEvents(filter RawEventFilter) ([]cfclient.Event, error) {
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
			coalesce(organization_guid::text, ''),
			coalesce(space_guid::text, ''),
			metadata
		from
			` + CFAuditEventsTable + `
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

func (s *EventStore) GetUnshippedCFAuditEventsForShipper(shipperName string) ([]cfclient.Event, error) {
	events := []cfclient.Event{}
	ctx, cancel := context.WithTimeout(s.ctx, DefaultQueryTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`
		with last_shipped_event as (
			select updated_at, shipped_id
			from
				` + ShipperCursorsTable + ` where name = '` + shipperName + `'
			union
				select (date '1970 1 1')::timestamptz, ''
			order by updated_at desc
			limit 1
		),
		recent_cf_audit_events as (
			select *
			from ` + CFAuditEventsTable + `
			where created_at >= (select updated_at from last_shipped_event)
			order by created_at asc
			limit 2048
		)
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
			coalesce(organization_guid::text, ''),
			coalesce(space_guid::text, ''),
			metadata
		from recent_cf_audit_events
		where guid::text != (select shipped_id from last_shipped_event)
		order by created_at asc
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

func (s *EventStore) UpdateShipperCursor(shipperName string, shipperTime string, shippedID string) error {
	ctx, cancel := context.WithTimeout(s.ctx, DefaultStoreTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt := fmt.Sprintf(
		`insert into %s (name, updated_at, shipped_id) values (
				$1, $2, $3
			) on conflict on constraint name_unique do
			update set
				updated_at = excluded.updated_at,
				shipped_id = excluded.shipped_id`,
		ShipperCursorsTable,
	)

	_, err = tx.Exec(stmt, shipperName, shipperTime, shippedID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *EventStore) GetLatestCFEventTime() (time.Time, error) {
	ctx, cancel := context.WithTimeout(s.ctx, DefaultQueryTimeout)
	defer cancel()
	row := s.db.QueryRowContext(ctx, `
		select
			created_at
		from
			`+CFAuditEventsTable+`
		order by
			created_at DESC
		limit 1
	`)

	createdAt := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	err := row.Scan(&createdAt)
	if err != nil && err != sql.ErrNoRows {
		return createdAt, err
	}
	return createdAt, nil // if no rows, return 1st Jan 1970
}

func (s *EventStore) GetCFEventCount() (int64, error) {
	ctx, cancel := context.WithTimeout(s.ctx, DefaultQueryTimeout)
	defer cancel()
	row := s.db.QueryRowContext(
		ctx,
		fmt.Sprintf(
			`SELECT reltuples::numeric FROM pg_class WHERE relname = '%s';`,
			CFAuditEventsTable,
		),
	)

	var cfEventCount int64
	err := row.Scan(&cfEventCount)
	if err == sql.ErrNoRows {
		return int64(0), nil
	} else if err != nil {
		return int64(0), err
	}
	return cfEventCount, nil
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
