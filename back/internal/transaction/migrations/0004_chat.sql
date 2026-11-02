-- Chat rooms are one table with three kinds: dm, project and open.
-- Project rooms are curated by the project lead; open rooms are created by an
-- auto-approved request for now, leaving the same rows ready for a later admin
-- review queue. Message bodies and attachments are soft-deleted by message.
CREATE TABLE chat_rooms (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    kind        TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    purpose     TEXT NOT NULL DEFAULT '',
    project_id  INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    created_by  INTEGER NOT NULL REFERENCES users(id),
    dm_key      TEXT,
    visibility  TEXT NOT NULL DEFAULT 'private',
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TEXT NOT NULL,
    archived_at TEXT
);

CREATE UNIQUE INDEX idx_chat_dm_key ON chat_rooms(dm_key) WHERE dm_key IS NOT NULL;
CREATE UNIQUE INDEX idx_chat_project_name
    ON chat_rooms(project_id, name COLLATE NOCASE)
    WHERE kind = 'project' AND status = 'active';
CREATE INDEX idx_chat_rooms_project ON chat_rooms(project_id);
CREATE INDEX idx_chat_rooms_status ON chat_rooms(kind, status);

CREATE TABLE chat_members (
    room_id              INTEGER NOT NULL REFERENCES chat_rooms(id) ON DELETE CASCADE,
    user_id              INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role                 TEXT NOT NULL DEFAULT 'member',
    joined_at            TEXT NOT NULL,
    last_read_message_id INTEGER,
    PRIMARY KEY (room_id, user_id)
);

CREATE INDEX idx_chat_members_user ON chat_members(user_id);

CREATE TABLE chat_messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id    INTEGER NOT NULL REFERENCES chat_rooms(id) ON DELETE CASCADE,
    sender_id  INTEGER NOT NULL REFERENCES users(id),
    body       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    edited_at  TEXT,
    deleted_at TEXT
);

CREATE INDEX idx_chat_messages_room ON chat_messages(room_id, id);
CREATE INDEX idx_chat_messages_sender ON chat_messages(sender_id);

CREATE TABLE chat_attachments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id     INTEGER NOT NULL REFERENCES chat_rooms(id) ON DELETE CASCADE,
    uploader_id INTEGER NOT NULL REFERENCES users(id),
    message_id  INTEGER REFERENCES chat_messages(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    filename    TEXT NOT NULL,
    mime        TEXT NOT NULL,
    size_bytes  INTEGER NOT NULL,
    width       INTEGER,
    height      INTEGER,
    created_at  TEXT NOT NULL
);

CREATE INDEX idx_chat_attachments_message ON chat_attachments(message_id);
CREATE INDEX idx_chat_attachments_room ON chat_attachments(room_id);

CREATE TABLE chat_requests (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    requester_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    purpose     TEXT NOT NULL,
    visibility  TEXT NOT NULL DEFAULT 'discoverable',
    status      TEXT NOT NULL DEFAULT 'approved',
    reviewed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    review_note TEXT,
    room_id     INTEGER REFERENCES chat_rooms(id) ON DELETE SET NULL,
    created_at  TEXT NOT NULL,
    reviewed_at TEXT
);

CREATE INDEX idx_chat_requests_requester ON chat_requests(requester_id);

CREATE TABLE chat_request_invites (
    request_id INTEGER NOT NULL REFERENCES chat_requests(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (request_id, user_id)
);
