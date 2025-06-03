CREATE TABLE IF NOT EXISTS kms_key_rotation (
    id SERIAL PRIMARY KEY,
    key_id TEXT NOT NULL,
    last_rotated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    next_rotation_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_kms_key_rotation_next_rotation ON kms_key_rotation(next_rotation_at); 