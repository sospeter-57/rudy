CREATE TABLE project_join_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    requester_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    created_at TEXT NOT NULL,
    reviewed_by INTEGER REFERENCES users(id),
    reviewed_at TEXT
);

CREATE INDEX idx_project_join_requests_project ON project_join_requests(project_id, status);
CREATE INDEX idx_project_join_requests_requester ON project_join_requests(requester_id);

CREATE UNIQUE INDEX idx_project_join_requests_pending
    ON project_join_requests(project_id, requester_id)
    WHERE status = 'pending';