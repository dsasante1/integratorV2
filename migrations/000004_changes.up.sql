 CREATE TABLE IF NOT EXISTS changes (
           id SERIAL PRIMARY KEY,
           collection_id TEXT NOT NULL,
           old_snapshot_id INTEGER,
           new_snapshot_id INTEGER NOT NULL,
           change_type TEXT NOT NULL,
           path TEXT NOT NULL,
           old_value TEXT,
           new_value TEXT,
           change_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
           created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
           updated_At TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
           FOREIGN KEY (collection_id) REFERENCES collections(id),
           FOREIGN KEY (old_snapshot_id) REFERENCES snapshots(id),
           FOREIGN KEY (new_snapshot_id) REFERENCES snapshots(id)
 );