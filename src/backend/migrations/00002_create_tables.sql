-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE spots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    address text NOT NULL,
    -- Maintained by hand, like is_closed.
    location geography(Point, 4326) NOT NULL,
    is_closed boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER spots_set_updated_at
    BEFORE UPDATE ON spots
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE appearances (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    spot_id uuid NOT NULL REFERENCES spots (id) ON DELETE CASCADE,
    youtube_id text NOT NULL,
    -- Seconds from the start of the video.
    "timestamp" integer NOT NULL CHECK ("timestamp" >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (spot_id, youtube_id, "timestamp")
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    google_sub text NOT NULL UNIQUE,
    email text NOT NULL,
    display_name text NOT NULL,
    profile_public boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE visits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    spot_id uuid NOT NULL REFERENCES spots (id),
    visited_at timestamptz NOT NULL DEFAULT now(),
    memo text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, spot_id, visited_at)
);

-- For counting the visitors of each spot.
CREATE INDEX visits_spot_id_idx ON visits (spot_id);

CREATE TRIGGER visits_set_updated_at
    BEFORE UPDATE ON visits
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
-- For deleting the expired sessions.
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
DROP TABLE visits;
DROP TABLE users;
DROP TABLE appearances;
DROP TABLE spots;
DROP FUNCTION set_updated_at();
