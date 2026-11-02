package transaction

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"rudy/internal/models"
)

// NewRoom is the insert shape shared by DMs, project rooms and open rooms.
type NewRoom struct {
	Kind       models.RoomKind
	Name       string
	Purpose    string
	ProjectID  *int64
	CreatedBy  int64
	DMKey      *string
	Visibility models.RoomVisibility
	Status     models.RoomStatus
	CreatedAt  time.Time
}

func nullTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed := parseTime(value.String)
	return &parsed
}

func nullInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	parsed := value.Int64
	return &parsed
}

func isMissing(err error) bool {
	var appErr *models.AppError
	return errors.As(err, &appErr) && appErr.Kind == models.KindNotFound
}

// ---------- project members ----------

func (r *Repo) ProjectMembers(ctx context.Context, projectID int64) ([]models.ProjectMember, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT pm.user_id, u.username, u.role, pm.role, pm.added_by, pm.joined_at
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = ?
		ORDER BY CASE WHEN pm.role = 'lead' THEN 0 ELSE 1 END, u.username ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []models.ProjectMember{}
	for rows.Next() {
		var (
			member   models.ProjectMember
			addedBy  sql.NullInt64
			joinedAt string
		)
		if err := rows.Scan(&member.UserID, &member.Username, &member.Role, &member.ProjectRole, &addedBy, &joinedAt); err != nil {
			return nil, err
		}
		member.AddedBy = nullInt64(addedBy)
		member.JoinedAt = parseTime(joinedAt)
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repo) AddProjectMember(ctx context.Context, projectID, userID int64, role string, addedBy int64, joinedAt time.Time) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role, added_by, joined_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(project_id, user_id) DO NOTHING`,
		projectID, userID, role, addedBy, formatTime(joinedAt),
	)
	return err
}

func (r *Repo) RemoveProjectMember(ctx context.Context, projectID, userID int64) error {
	result, err := r.q.ExecContext(ctx,
		`DELETE FROM project_members WHERE project_id = ? AND user_id = ?`, projectID, userID)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "project member not found")
}

func (r *Repo) IsProjectMember(ctx context.Context, projectID, userID int64) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_members WHERE project_id = ? AND user_id = ?`,
		projectID, userID).Scan(&count)
	return count > 0, err
}

// RemoveMemberFromProjectChats removes a user from every room that belongs to a
// project, so leaving a project immediately revokes chat access.
func (r *Repo) RemoveMemberFromProjectChats(ctx context.Context, projectID, userID int64) error {
	_, err := r.q.ExecContext(ctx, `
		DELETE FROM chat_members
		WHERE user_id = ? AND room_id IN (SELECT id FROM chat_rooms WHERE project_id = ?)`,
		userID, projectID,
	)
	return err
}

// ---------- rooms ----------

const roomColumns = `
	r.id, r.kind, r.name, r.purpose, r.project_id, COALESCE(p.name, ''),
	r.created_by, r.visibility, r.status, r.created_at, r.archived_at`

func scanRoom(row scanner) (models.ChatRoom, error) {
	var (
		room       models.ChatRoom
		projectID  sql.NullInt64
		createdAt  string
		archivedAt sql.NullString
	)
	err := row.Scan(
		&room.ID, &room.Kind, &room.Name, &room.Purpose, &projectID, &room.ProjectName,
		&room.CreatedBy, &room.Visibility, &room.Status, &createdAt, &archivedAt,
	)
	if err != nil {
		return models.ChatRoom{}, err
	}
	room.ProjectID = nullInt64(projectID)
	room.CreatedAt = parseTime(createdAt)
	room.ArchivedAt = nullTime(archivedAt)
	return room, nil
}

func (r *Repo) CreateChatRoom(ctx context.Context, input NewRoom) (models.ChatRoom, error) {
	var projectID any
	if input.ProjectID != nil {
		projectID = *input.ProjectID
	}
	var dmKey any
	if input.DMKey != nil {
		dmKey = *input.DMKey
	}

	var id int64
	err := r.q.QueryRowContext(ctx, `
		INSERT INTO chat_rooms (kind, name, purpose, project_id, created_by, dm_key, visibility, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		input.Kind, input.Name, input.Purpose, projectID, input.CreatedBy, dmKey,
		input.Visibility, input.Status, formatTime(input.CreatedAt),
	).Scan(&id)
	if err != nil {
		return models.ChatRoom{}, err
	}
	return r.ChatRoomByID(ctx, id)
}

func (r *Repo) ChatRoomByID(ctx context.Context, id int64) (models.ChatRoom, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT `+roomColumns+`
		FROM chat_rooms r
		LEFT JOIN projects p ON p.id = r.project_id
		WHERE r.id = ?`, id)
	room, err := scanRoom(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ChatRoom{}, models.NotFound("chat room not found")
	}
	return room, err
}

func (r *Repo) ChatRoomByDMKey(ctx context.Context, key string) (models.ChatRoom, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT `+roomColumns+`
		FROM chat_rooms r
		LEFT JOIN projects p ON p.id = r.project_id
		WHERE r.dm_key = ?`, key)
	room, err := scanRoom(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ChatRoom{}, models.NotFound("chat room not found")
	}
	return room, err
}

func (r *Repo) ChatRoomsForUser(ctx context.Context, userID int64) ([]models.ChatRoom, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT `+roomColumns+`,
			(SELECT COUNT(*) FROM chat_messages m
			 WHERE m.room_id = r.id AND m.sender_id <> ? AND m.deleted_at IS NULL
			   AND (cm.last_read_message_id IS NULL OR m.id > cm.last_read_message_id)) AS unread,
			(SELECT MAX(m.id) FROM chat_messages m WHERE m.room_id = r.id) AS last_id
		FROM chat_rooms r
		JOIN chat_members cm ON cm.room_id = r.id AND cm.user_id = ?
		LEFT JOIN projects p ON p.id = r.project_id
		WHERE r.status = 'active'
		ORDER BY COALESCE((SELECT MAX(m.created_at) FROM chat_messages m WHERE m.room_id = r.id), r.created_at) DESC`,
		userID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rooms := []models.ChatRoom{}
	lastIDs := []int64{}
	for rows.Next() {
		var (
			room       models.ChatRoom
			projectID  sql.NullInt64
			createdAt  string
			archivedAt sql.NullString
			lastID     sql.NullInt64
			unread     int
		)
		if err := rows.Scan(
			&room.ID, &room.Kind, &room.Name, &room.Purpose, &projectID, &room.ProjectName,
			&room.CreatedBy, &room.Visibility, &room.Status, &createdAt, &archivedAt, &unread, &lastID,
		); err != nil {
			return nil, err
		}
		room.ProjectID = nullInt64(projectID)
		room.CreatedAt = parseTime(createdAt)
		room.ArchivedAt = nullTime(archivedAt)
		room.UnreadCount = unread
		rooms = append(rooms, room)
		if lastID.Valid {
			lastIDs = append(lastIDs, lastID.Int64)
		} else {
			lastIDs = append(lastIDs, 0)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Close before loading last messages: the pool has one connection, so a
	// nested query while rows are open would wait forever.
	rows.Close()

	for i, lastID := range lastIDs {
		if lastID == 0 {
			continue
		}
		message, err := r.ChatMessageByID(ctx, lastID)
		if err != nil && !isMissing(err) {
			return nil, err
		}
		if err == nil {
			rooms[i].LastMessage = &message
		}
	}
	return rooms, nil
}

func (r *Repo) ProjectChatRooms(ctx context.Context, projectID int64) ([]models.ChatRoom, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT `+roomColumns+`
		FROM chat_rooms r
		LEFT JOIN projects p ON p.id = r.project_id
		WHERE r.project_id = ? AND r.kind = 'project'
		ORDER BY CASE WHEN r.status = 'active' THEN 0 ELSE 1 END, r.created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rooms := []models.ChatRoom{}
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (r *Repo) DiscoverChatRooms(ctx context.Context, userID int64) ([]models.ChatRoom, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT `+roomColumns+`
		FROM chat_rooms r
		LEFT JOIN projects p ON p.id = r.project_id
		WHERE r.kind = 'open' AND r.visibility = 'discoverable' AND r.status = 'active'
		  AND r.id NOT IN (SELECT room_id FROM chat_members WHERE user_id = ?)
		ORDER BY r.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rooms := []models.ChatRoom{}
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (r *Repo) UpdateChatRoom(ctx context.Context, id int64, name, purpose string, visibility models.RoomVisibility) error {
	result, err := r.q.ExecContext(ctx, `
		UPDATE chat_rooms SET name = ?, purpose = ?, visibility = ? WHERE id = ? AND status = 'active'`,
		name, purpose, visibility, id,
	)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "chat room not found")
}

func (r *Repo) ArchiveChatRoom(ctx context.Context, id int64, archivedAt time.Time) error {
	result, err := r.q.ExecContext(ctx, `
		UPDATE chat_rooms SET status = 'archived', archived_at = ? WHERE id = ? AND status = 'active'`,
		formatTime(archivedAt), id,
	)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "chat room not found")
}

// ---------- room members ----------

func (r *Repo) ChatRoomMembers(ctx context.Context, roomID int64) ([]models.ChatMember, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT cm.user_id, u.username, u.role, cm.role, cm.joined_at
		FROM chat_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.room_id = ?
		ORDER BY CASE WHEN cm.role = 'owner' THEN 0 ELSE 1 END, u.username ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []models.ChatMember{}
	for rows.Next() {
		var (
			member   models.ChatMember
			joinedAt string
		)
		if err := rows.Scan(&member.UserID, &member.Username, &member.Role, &member.RoomRole, &joinedAt); err != nil {
			return nil, err
		}
		member.JoinedAt = parseTime(joinedAt)
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repo) AddChatMember(ctx context.Context, roomID, userID int64, role string, joinedAt time.Time) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO chat_members (room_id, user_id, role, joined_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(room_id, user_id) DO NOTHING`,
		roomID, userID, role, formatTime(joinedAt),
	)
	return err
}

func (r *Repo) RemoveChatMember(ctx context.Context, roomID, userID int64) error {
	result, err := r.q.ExecContext(ctx,
		`DELETE FROM chat_members WHERE room_id = ? AND user_id = ?`, roomID, userID)
	if err != nil {
		return err
	}
	return r.requireAffected(result, "room member not found")
}

func (r *Repo) IsChatMember(ctx context.Context, roomID, userID int64) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_members WHERE room_id = ? AND user_id = ?`,
		roomID, userID).Scan(&count)
	return count > 0, err
}

func (r *Repo) ChatRoomMemberIDs(ctx context.Context, roomID int64) ([]int64, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT user_id FROM chat_members WHERE room_id = ?`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repo) ChatMemberRole(ctx context.Context, roomID, userID int64) (string, error) {
	var role string
	err := r.q.QueryRowContext(ctx,
		`SELECT role FROM chat_members WHERE room_id = ? AND user_id = ?`, roomID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", models.NotFound("room member not found")
	}
	return role, err
}

func (r *Repo) MarkChatRead(ctx context.Context, roomID, userID, messageID int64) error {
	result, err := r.q.ExecContext(ctx, `
		UPDATE chat_members SET last_read_message_id = ?
		WHERE room_id = ? AND user_id = ? AND (last_read_message_id IS NULL OR last_read_message_id < ?)`,
		messageID, roomID, userID, messageID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Already at or beyond this message is fine; a missing membership is not.
		var count int
		if err := r.q.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM chat_members WHERE room_id = ? AND user_id = ?`,
			roomID, userID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return models.NotFound("room member not found")
		}
	}
	return nil
}

// ---------- messages ----------

func (r *Repo) CreateChatMessage(ctx context.Context, roomID, senderID int64, body string, createdAt time.Time) (models.ChatMessage, error) {
	var id int64
	err := r.q.QueryRowContext(ctx, `
		INSERT INTO chat_messages (room_id, sender_id, body, created_at)
		VALUES (?, ?, ?, ?)
		RETURNING id`, roomID, senderID, body, formatTime(createdAt)).Scan(&id)
	if err != nil {
		return models.ChatMessage{}, err
	}
	return r.ChatMessageByID(ctx, id)
}

func (r *Repo) ChatMessageByID(ctx context.Context, id int64) (models.ChatMessage, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT m.id, m.room_id, m.sender_id, u.username, m.body, m.created_at, m.edited_at, m.deleted_at
		FROM chat_messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.id = ?`, id)

	var (
		message   models.ChatMessage
		createdAt string
		editedAt  sql.NullString
		deletedAt sql.NullString
	)
	err := row.Scan(
		&message.ID, &message.RoomID, &message.SenderID, &message.SenderName,
		&message.Body, &createdAt, &editedAt, &deletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ChatMessage{}, models.NotFound("message not found")
	}
	if err != nil {
		return models.ChatMessage{}, err
	}
	message.CreatedAt = parseTime(createdAt)
	message.EditedAt = nullTime(editedAt)
	message.DeletedAt = nullTime(deletedAt)
	message.Attachments = []models.ChatAttachment{}

	if message.DeletedAt == nil {
		attachments, err := r.ChatAttachmentsForMessages(ctx, []int64{message.ID})
		if err != nil {
			return models.ChatMessage{}, err
		}
		message.Attachments = attachments
	} else {
		// Soft-deleted messages keep their row for auditability but hide content.
		message.Body = ""
	}
	if message.Attachments == nil {
		message.Attachments = []models.ChatAttachment{}
	}

	return message, nil
}

// ChatMessages returns a page in ascending order. Pass the oldest message id in
// the current page as `before` to walk backwards.
func (r *Repo) ChatMessages(ctx context.Context, roomID int64, before *int64, limit int) ([]models.ChatMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT m.id, m.room_id, m.sender_id, u.username, m.body, m.created_at, m.edited_at, m.deleted_at
		FROM chat_messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.room_id = ?`
	args := []any{roomID}
	if before != nil {
		query += ` AND m.id < ?`
		args = append(args, *before)
	}
	query += ` ORDER BY m.id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []models.ChatMessage{}
	for rows.Next() {
		var (
			message   models.ChatMessage
			createdAt string
			editedAt  sql.NullString
			deletedAt sql.NullString
		)
		if err := rows.Scan(
			&message.ID, &message.RoomID, &message.SenderID, &message.SenderName,
			&message.Body, &createdAt, &editedAt, &deletedAt,
		); err != nil {
			return nil, err
		}
		message.CreatedAt = parseTime(createdAt)
		message.EditedAt = nullTime(editedAt)
		message.DeletedAt = nullTime(deletedAt)
		message.Attachments = []models.ChatAttachment{}
		if message.DeletedAt != nil {
			message.Body = ""
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	ids := make([]int64, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	attachments, err := r.ChatAttachmentsForMessages(ctx, ids)
	if err != nil {
		return nil, err
	}
	byMessage := map[int64][]models.ChatAttachment{}
	for _, attachment := range attachments {
		if attachment.MessageID == nil {
			continue
		}
		byMessage[*attachment.MessageID] = append(byMessage[*attachment.MessageID], attachment)
	}
	for i := range messages {
		if list, ok := byMessage[messages[i].ID]; ok {
			messages[i].Attachments = list
		}
	}

	// Reverse to ascending chronological order for the client.
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (r *Repo) UpdateChatMessage(ctx context.Context, id int64, body string, editedAt time.Time) (models.ChatMessage, error) {
	result, err := r.q.ExecContext(ctx, `
		UPDATE chat_messages SET body = ?, edited_at = ?
		WHERE id = ? AND deleted_at IS NULL`, body, formatTime(editedAt), id)
	if err != nil {
		return models.ChatMessage{}, err
	}
	if err := r.requireAffected(result, "message not found"); err != nil {
		return models.ChatMessage{}, err
	}
	return r.ChatMessageByID(ctx, id)
}

func (r *Repo) SoftDeleteChatMessage(ctx context.Context, id int64, deletedAt time.Time) (models.ChatMessage, error) {
	result, err := r.q.ExecContext(ctx, `
		UPDATE chat_messages SET deleted_at = ?
		WHERE id = ? AND deleted_at IS NULL`, formatTime(deletedAt), id)
	if err != nil {
		return models.ChatMessage{}, err
	}
	if err := r.requireAffected(result, "message not found"); err != nil {
		return models.ChatMessage{}, err
	}
	return r.ChatMessageByID(ctx, id)
}

func (r *Repo) MessageExistsInRoom(ctx context.Context, roomID, messageID int64) (bool, error) {
	var count int
	err := r.q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_messages WHERE id = ? AND room_id = ?`, messageID, roomID).Scan(&count)
	return count > 0, err
}

// ---------- attachments ----------

const attachmentColumns = `
	a.id, a.room_id, a.uploader_id, a.message_id, a.storage_key, a.filename,
	a.mime, a.size_bytes, a.width, a.height, a.created_at`

func scanAttachment(row scanner) (models.ChatAttachment, error) {
	var (
		attachment models.ChatAttachment
		messageID  sql.NullInt64
		width      sql.NullInt64
		height     sql.NullInt64
		createdAt  string
	)
	err := row.Scan(
		&attachment.ID, &attachment.RoomID, &attachment.UploaderID, &messageID, &attachment.StorageKey, &attachment.Filename,
		&attachment.Mime, &attachment.SizeBytes, &width, &height, &createdAt,
	)
	if err != nil {
		return models.ChatAttachment{}, err
	}
	attachment.MessageID = nullInt64(messageID)
	if width.Valid {
		value := int(width.Int64)
		attachment.Width = &value
	}
	if height.Valid {
		value := int(height.Int64)
		attachment.Height = &value
	}
	attachment.CreatedAt = parseTime(createdAt)
	return attachment, nil
}

func (r *Repo) CreateChatAttachment(ctx context.Context, roomID, uploaderID int64, storageKey, filename, mime string, sizeBytes int64, width, height *int, createdAt time.Time) (models.ChatAttachment, error) {
	var widthValue, heightValue any
	if width != nil {
		widthValue = *width
	}
	if height != nil {
		heightValue = *height
	}

	var id int64
	err := r.q.QueryRowContext(ctx, `
		INSERT INTO chat_attachments
			(room_id, uploader_id, storage_key, filename, mime, size_bytes, width, height, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		roomID, uploaderID, storageKey, filename, mime, sizeBytes, widthValue, heightValue, formatTime(createdAt),
	).Scan(&id)
	if err != nil {
		return models.ChatAttachment{}, err
	}
	return r.ChatAttachmentByID(ctx, id)
}

func (r *Repo) ChatAttachmentByID(ctx context.Context, id int64) (models.ChatAttachment, error) {
	row := r.q.QueryRowContext(ctx, `
		SELECT `+attachmentColumns+`
		FROM chat_attachments a WHERE a.id = ?`, id)
	attachment, err := scanAttachment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ChatAttachment{}, models.NotFound("attachment not found")
	}
	return attachment, err
}

// ChatAttachmentUploader is used to check ownership before attaching uploads.
func (r *Repo) ChatAttachmentUploader(ctx context.Context, id int64) (roomID, uploaderID int64, messageID *int64, err error) {
	var (
		nullMessage sql.NullInt64
	)
	err = r.q.QueryRowContext(ctx,
		`SELECT room_id, uploader_id, message_id FROM chat_attachments WHERE id = ?`, id).
		Scan(&roomID, &uploaderID, &nullMessage)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil, models.NotFound("attachment not found")
	}
	if err != nil {
		return 0, 0, nil, err
	}
	return roomID, uploaderID, nullInt64(nullMessage), nil
}

func (r *Repo) AttachChatAttachments(ctx context.Context, messageID, roomID, uploaderID int64, ids []int64) error {
	for _, id := range ids {
		result, err := r.q.ExecContext(ctx, `
			UPDATE chat_attachments SET message_id = ?
			WHERE id = ? AND room_id = ? AND uploader_id = ? AND message_id IS NULL`,
			messageID, id, roomID, uploaderID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return models.Conflict("an attachment was already used or does not belong to this room")
		}
	}
	return nil
}

func (r *Repo) ChatAttachmentsForMessages(ctx context.Context, messageIDs []int64) ([]models.ChatAttachment, error) {
	if len(messageIDs) == 0 {
		return []models.ChatAttachment{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(messageIDs)), ",")
	args := make([]any, len(messageIDs))
	for i, id := range messageIDs {
		args[i] = id
	}
	rows, err := r.q.QueryContext(ctx, `
		SELECT `+attachmentColumns+`
		FROM chat_attachments a
		WHERE a.message_id IN (`+placeholders+`)
		ORDER BY a.id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := []models.ChatAttachment{}
	for rows.Next() {
		attachment, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}

// ---------- requests ----------

func (r *Repo) CreateChatRequest(ctx context.Context, requesterID int64, name, purpose string, visibility models.RoomVisibility, status string, roomID int64, reviewedAt, createdAt time.Time) (int64, error) {
	var id int64
	err := r.q.QueryRowContext(ctx, `
		INSERT INTO chat_requests
			(requester_id, name, purpose, visibility, status, room_id, created_at, reviewed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		requesterID, name, purpose, visibility, status, roomID, formatTime(createdAt), formatTime(reviewedAt),
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repo) AddChatRequestInvite(ctx context.Context, requestID, userID int64) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO chat_request_invites (request_id, user_id) VALUES (?, ?)
		ON CONFLICT(request_id, user_id) DO NOTHING`, requestID, userID)
	return err
}

func (r *Repo) ChatRequestsForUser(ctx context.Context, userID int64) ([]models.ChatRequest, error) {
	rows, err := r.q.QueryContext(ctx, `
		SELECT q.id, q.requester_id, u.username, q.name, q.purpose, q.visibility, q.status,
			q.room_id, q.created_at, q.reviewed_at
		FROM chat_requests q
		JOIN users u ON u.id = q.requester_id
		WHERE q.requester_id = ?
		ORDER BY q.created_at DESC, q.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := []models.ChatRequest{}
	for rows.Next() {
		var (
			request    models.ChatRequest
			roomID     sql.NullInt64
			createdAt  string
			reviewedAt sql.NullString
		)
		if err := rows.Scan(
			&request.ID, &request.RequesterID, &request.RequesterName, &request.Name,
			&request.Purpose, &request.Visibility, &request.Status, &roomID, &createdAt, &reviewedAt,
		); err != nil {
			return nil, err
		}
		request.RoomID = nullInt64(roomID)
		request.CreatedAt = parseTime(createdAt)
		request.ReviewedAt = nullTime(reviewedAt)
		request.Invites = []int64{}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	for i := range requests {
		invites, err := r.ChatRequestInvites(ctx, requests[i].ID)
		if err != nil {
			return nil, err
		}
		requests[i].Invites = invites
	}
	return requests, nil
}

func (r *Repo) ChatRequestInvites(ctx context.Context, requestID int64) ([]int64, error) {
	rows, err := r.q.QueryContext(ctx,
		`SELECT user_id FROM chat_request_invites WHERE request_id = ? ORDER BY user_id`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
