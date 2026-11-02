-- Rename the global manager role to Lead. The per-project lead is separate:
-- it is always projects.creator_id, recorded as the project_members lead row.
UPDATE users SET role = 'Lead' WHERE role = 'Project Manager';

CREATE TABLE project_members (
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member',
    added_by   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    joined_at  TEXT NOT NULL,
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_members_user ON project_members(user_id);

INSERT INTO project_members (project_id, user_id, role, joined_at)
SELECT id, creator_id, 'lead', created_at FROM projects;
