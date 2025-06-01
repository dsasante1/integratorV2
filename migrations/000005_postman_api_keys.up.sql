  CREATE TABLE IF NOT EXISTS postman_api_keys (
           id SERIAL PRIMARY KEY,
           user_id INTEGER NOT NULL,
           api_key TEXT NOT NULL,
           created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
           last_used_at TIMESTAMP WITH TIME ZONE,
           FOREIGN KEY (user_id) REFERENCES users(id)
    );