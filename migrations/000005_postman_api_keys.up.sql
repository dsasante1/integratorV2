CREATE TABLE IF NOT EXISTS postman_api_keys (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    encrypted_key TEXT NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    expires_at TIMESTAMP WITH TIME ZONE,
    last_rotated_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);


CREATE INDEX idx_postman_api_keys_user_id ON postman_api_keys(user_id);
CREATE INDEX idx_postman_api_keys_is_active ON postman_api_keys(is_active);
CREATE INDEX idx_postman_api_keys_expires_at ON postman_api_keys(expires_at);