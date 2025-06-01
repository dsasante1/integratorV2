CREATE TABLE IF NOT EXISTS snapshots (
           id SERIAL PRIMARY KEY,
           collection_id TEXT NOT NULL,
           snapshot_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
           content JSONB NOT NULL,
           hash TEXT NOT NULL,
           FOREIGN KEY (collection_id) REFERENCES collections(id)
    );