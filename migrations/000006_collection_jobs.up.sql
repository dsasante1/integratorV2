CREATE TABLE IF NOT EXISTS collection_jobs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    collection_id TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_collection_jobs_user_id ON collection_jobs(user_id);
CREATE INDEX idx_collection_jobs_status ON collection_jobs(status);
CREATE INDEX idx_collection_jobs_collection_id ON collection_jobs(collection_id); 