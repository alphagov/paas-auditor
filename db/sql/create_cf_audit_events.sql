CREATE TABLE IF NOT EXISTS cf_audit_events (
	id SERIAL, -- this should probably be called "sequence" it's not really an id
	guid uuid UNIQUE NOT NULL,
	created_at timestamptz NOT NULL,
	raw_message JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS cf_audit_events_id_idx ON cf_audit_events (id);
CREATE INDEX IF NOT EXISTS cf_audit_events_state_organization_guid_idx ON cf_audit_events ( (raw_message->>'organization_guid') );
CREATE INDEX IF NOT EXISTS cf_audit_events_state_space_guid_idx ON cf_audit_events ( (raw_message->>'space_guid') );
CREATE INDEX IF NOT EXISTS cf_audit_events_state_type_idx ON cf_audit_events ( (raw_message->>'type') );

DO $$ BEGIN
	ALTER TABLE cf_audit_events ADD CONSTRAINT created_at_not_zero_value CHECK (created_at > 'epoch'::timestamptz);
EXCEPTION
	WHEN duplicate_object THEN RAISE NOTICE 'constraint already exists';
END; $$;
