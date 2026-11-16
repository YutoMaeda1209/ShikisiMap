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
    -- NULL until the place is resolved with the Places API.
    place_id text UNIQUE,
    name text NOT NULL,
    address text NOT NULL,
    location geography(Point, 4326) NOT NULL,
    -- NULL while the location is a provisional one that did not come from Google.
    location_fetched_at timestamptz,
    is_closed boolean NOT NULL DEFAULT false,
    status_checked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- The scheduled job picks the spots whose location is the oldest.
CREATE INDEX spots_location_fetched_at_idx ON spots (location_fetched_at);

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
