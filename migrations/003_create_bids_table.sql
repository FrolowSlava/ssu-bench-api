-- Create bids table
CREATE TABLE IF NOT EXISTS bids (
    id SERIAL PRIMARY KEY,
    task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    executor_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(task_id, executor_id)
    );

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_bids_task_id ON bids(task_id);
CREATE INDEX IF NOT EXISTS idx_bids_executor_id ON bids(executor_id);
CREATE INDEX IF NOT EXISTS idx_bids_status ON bids(status);