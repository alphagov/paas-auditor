CREATE TABLE IF NOT EXISTS cf_audit_events (
	id SERIAL, -- this should probably be called "sequence" it's not really an id
	guid uuid UNIQUE NOT NULL,
	created_at timestamptz NOT NULL,
	event_type text NOT NULL,
	actor text NOT NULL,
	actor_type text NOT NULL,
	actor_name text NOT NULL,
	actor_username text NOT NULL,
	actee text NOT NULL,
	actee_type text NOT NULL,
	actee_name text NOT NULL,
	organization_guid uuid,
	space_guid uuid,

	PRIMARY KEY (guid)
);

CREATE INDEX IF NOT EXISTS cf_audit_events_id_idx ON cf_audit_events (id);
CREATE INDEX IF NOT EXISTS cf_audit_events_guid_idx ON cf_audit_events (guid);
CREATE INDEX IF NOT EXISTS cf_audit_events_state_organization_guid_idx ON cf_audit_events (organization_guid);
CREATE INDEX IF NOT EXISTS cf_audit_events_state_space_guid_idx ON cf_audit_events (space_guid);
CREATE INDEX IF NOT EXISTS cf_audit_events_state_event_type_idx ON cf_audit_events (event_type);

DO $$ BEGIN
	ALTER TABLE cf_audit_events ADD CONSTRAINT created_at_not_zero_value CHECK (created_at > 'epoch'::timestamptz);
EXCEPTION
	WHEN duplicate_object THEN RAISE NOTICE 'constraint already exists';
END; $$;

ALTER TABLE cf_audit_events ADD COLUMN IF NOT EXISTS metadata JSONB;
