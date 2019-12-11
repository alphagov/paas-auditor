CREATE TABLE IF NOT EXISTS shipper_cursors (
  name text UNIQUE NOT NULL,
	updated_at timestamptz NOT NULL,
	shipped_id text NOT NULL,
	PRIMARY KEY (name)
);

DO $$ BEGIN
	ALTER TABLE shipper_cursors ADD CONSTRAINT updated_at_not_zero_value CHECK (updated_at > 'epoch'::timestamptz);
EXCEPTION
	WHEN duplicate_object THEN RAISE NOTICE 'constraint already exists';
END; $$;
