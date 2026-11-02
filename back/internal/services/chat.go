package services

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"rudy/internal/events"
	"rudy/internal/models"
	"rudy/internal/storage"
	"rudy/internal/transaction"
)

const (
	maxMessageRunes = 4000
	maxRoomName     = 48
	maxPurposeRunes = 280
	maxAttachments  = 4
)

type ChatService struct {
	db    *transaction.DB
	store storage.AttachmentStore
	hub   *events.Hub
}

func NewChatService(db *transaction.DB, store storage.AttachmentStore, hub *events.Hub) *ChatService {
	if hub == nil {
		hub = events.NewHub()
	}
	return &ChatService{db: db, store: store, hub: hub}
}

func (s *ChatService) Hub() *events.Hub {
	return s.hub
}

type ProjectRoomInput struct {
	Name      string
	Purpose   string
	MemberIDs []int64
}

type OpenRoomInput struct {
	Name       string
	Purpose    string
	Visibility models.RoomVisibility
	InviteIDs  []int64
}

type OpenRoomResult struct {
	Room    models.ChatRoom    `json:"room"`
	Request models.ChatRequest `json:"request"`
}

// ---------- reading ----------

func (s *ChatService) ListRooms(ctx context.Context, actor models.User) ([]models.ChatRoom, error) {
	rooms, err := s.db.Repo().ChatRoomsForUser(ctx, actor.ID)
	if err != nil {
		return nil, err
	}
	for i := range rooms {
		members, err := s.db.Repo().ChatRoomMembers(ctx, rooms[i].ID)
		if err != nil {
			return nil, err
		}
		rooms[i].Members = members
	}
	return rooms, nil
}

func (s *ChatService) GetRoom(ctx context.Context, actor models.User, roomID int64) (models.ChatRoom, error) {
	room, err := s.requireMember(ctx, roomID, actor.ID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	members, err := s.db.Repo().ChatRoomMembers(ctx, roomID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	room.Members = members
	return room, nil
}

func (s *ChatService) ProjectRooms(ctx context.Context, actor models.User, projectID int64) ([]models.ChatRoom, error) {
	if _, err := s.db.Repo().ProjectByID(ctx, projectID); err != nil {
		return nil, err
	}
	member, err := s.db.Repo().IsProjectMember(ctx, projectID, actor.ID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, models.Forbidden("only project members can see project chats")
	}
	rooms, err := s.db.Repo().ProjectChatRooms(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for i := range rooms {
		members, err := s.db.Repo().ChatRoomMembers(ctx, rooms[i].ID)
		if err != nil {
			return nil, err
		}
		rooms[i].Members = members
	}
	return rooms, nil
}

func (s *ChatService) Discover(ctx context.Context, actor models.User) ([]models.ChatRoom, error) {
	rooms, err := s.db.Repo().DiscoverChatRooms(ctx, actor.ID)
	if err != nil {
		return nil, err
	}
	for i := range rooms {
		members, err := s.db.Repo().ChatRoomMembers(ctx, rooms[i].ID)
		if err != nil {
			return nil, err
		}
		rooms[i].Members = members
	}
	return rooms, nil
}

func (s *ChatService) Messages(ctx context.Context, actor models.User, roomID int64, before *int64, limit int) ([]models.ChatMessage, error) {
	if _, err := s.requireMember(ctx, roomID, actor.ID); err != nil {
		return nil, err
	}
	return s.db.Repo().ChatMessages(ctx, roomID, before, limit)
}

func (s *ChatService) UnreadTotal(ctx context.Context, actor models.User) (int, error) {
	rooms, err := s.db.Repo().ChatRoomsForUser(ctx, actor.ID)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, room := range rooms {
		total += room.UnreadCount
	}
	return total, nil
}

// ---------- direct messages ----------

func (s *ChatService) OpenDM(ctx context.Context, actor models.User, otherID int64) (models.ChatRoom, error) {
	if otherID == actor.ID {
		return models.ChatRoom{}, models.Validation("you cannot open a direct message with yourself")
	}
	exists, err := s.db.Repo().UserExists(ctx, otherID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if !exists {
		return models.ChatRoom{}, models.NotFound("user not found")
	}

	key := dmKey(actor.ID, otherID)
	if room, err := s.db.Repo().ChatRoomByDMKey(ctx, key); err == nil {
		return s.GetRoom(ctx, actor, room.ID)
	} else if !isNotFound(err) {
		return models.ChatRoom{}, err
	}

	now := nowUTC()
	var room models.ChatRoom
	err = s.db.WithTx(ctx, func(repo *transaction.Repo) error {
		created, err := repo.CreateChatRoom(ctx, transaction.NewRoom{
			Kind:       models.RoomDM,
			CreatedBy:  actor.ID,
			DMKey:      &key,
			Visibility: models.RoomPrivate,
			Status:     models.RoomActive,
			CreatedAt:  now,
		})
		if err != nil {
			return err
		}
		room = created
		if err := repo.AddChatMember(ctx, room.ID, actor.ID, "member", now); err != nil {
			return err
		}
		return repo.AddChatMember(ctx, room.ID, otherID, "member", now)
	})
	if err != nil {
		// A concurrent request may have created the same DM first.
		if existing, lookupErr := s.db.Repo().ChatRoomByDMKey(ctx, key); lookupErr == nil {
			return s.GetRoom(ctx, actor, existing.ID)
		}
		return models.ChatRoom{}, err
	}

	s.publishRoom(ctx, room.ID, "room.created", room)
	return s.GetRoom(ctx, actor, room.ID)
}

// ---------- room creation and management ----------

func (s *ChatService) CreateProjectRoom(ctx context.Context, actor models.User, projectID int64, input ProjectRoomInput) (models.ChatRoom, error) {
	project, err := s.db.Repo().ProjectByID(ctx, projectID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if project.CreatorID != actor.ID {
		return models.ChatRoom{}, models.Forbidden("only the project lead can create project chats")
	}
	name, purpose, err := roomFields(input.Name, input.Purpose, true)
	if err != nil {
		return models.ChatRoom{}, err
	}

	memberIDs, err := s.validateProjectRoomMembers(ctx, projectID, actor.ID, input.MemberIDs)
	if err != nil {
		return models.ChatRoom{}, err
	}

	now := nowUTC()
	var room models.ChatRoom
	err = s.db.WithTx(ctx, func(repo *transaction.Repo) error {
		created, err := repo.CreateChatRoom(ctx, transaction.NewRoom{
			Kind:       models.RoomProject,
			Name:       name,
			Purpose:    purpose,
			ProjectID:  &projectID,
			CreatedBy:  actor.ID,
			Visibility: models.RoomPrivate,
			Status:     models.RoomActive,
			CreatedAt:  now,
		})
		if err != nil {
			if isUniqueViolation(err) {
				return models.Conflict("a project chat with that name already exists")
			}
			return err
		}
		room = created
		if err := repo.AddChatMember(ctx, room.ID, actor.ID, "owner", now); err != nil {
			return err
		}
		for _, userID := range memberIDs {
			if err := repo.AddChatMember(ctx, room.ID, userID, "member", now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.ChatRoom{}, err
	}

	s.publishRoom(ctx, room.ID, "room.created", room)
	return s.GetRoom(ctx, actor, room.ID)
}

func (s *ChatService) CreateOpenRoom(ctx context.Context, actor models.User, input OpenRoomInput) (OpenRoomResult, error) {
	name, purpose, err := roomFields(input.Name, input.Purpose, true)
	if err != nil {
		return OpenRoomResult{}, err
	}
	visibility := input.Visibility
	if visibility == "" {
		visibility = models.RoomDiscoverable
	}
	if !models.IsRoomVisibility(visibility) {
		return OpenRoomResult{}, models.Validation("visibility must be private or discoverable")
	}

	invites, err := s.uniqueExistingUsers(ctx, actor.ID, input.InviteIDs)
	if err != nil {
		return OpenRoomResult{}, err
	}

	now := nowUTC()
	var room models.ChatRoom
	var requestID int64
	err = s.db.WithTx(ctx, func(repo *transaction.Repo) error {
		created, err := repo.CreateChatRoom(ctx, transaction.NewRoom{
			Kind:       models.RoomOpen,
			Name:       name,
			Purpose:    purpose,
			CreatedBy:  actor.ID,
			Visibility: visibility,
			Status:     models.RoomActive,
			CreatedAt:  now,
		})
		if err != nil {
			return err
		}
		room = created
		if err := repo.AddChatMember(ctx, room.ID, actor.ID, "owner", now); err != nil {
			return err
		}
		for _, userID := range invites {
			if err := repo.AddChatMember(ctx, room.ID, userID, "member", now); err != nil {
				return err
			}
		}

		// No admin exists yet, so the request is recorded as already approved.
		// The future admin queue can read the same rows and switch this to
		// pending review without changing room or message data.
		requestID, err = repo.CreateChatRequest(
			ctx, actor.ID, name, purpose, visibility, "approved", room.ID, now, now,
		)
		if err != nil {
			return err
		}
		for _, userID := range invites {
			if err := repo.AddChatRequestInvite(ctx, requestID, userID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return OpenRoomResult{}, err
	}

	s.publishRoom(ctx, room.ID, "room.created", room)
	full, err := s.GetRoom(ctx, actor, room.ID)
	if err != nil {
		return OpenRoomResult{}, err
	}
	requests, err := s.db.Repo().ChatRequestsForUser(ctx, actor.ID)
	if err != nil {
		return OpenRoomResult{}, err
	}
	var request models.ChatRequest
	for _, candidate := range requests {
		if candidate.ID == requestID {
			request = candidate
			break
		}
	}
	return OpenRoomResult{Room: full, Request: request}, nil
}

func (s *ChatService) UpdateRoom(ctx context.Context, actor models.User, roomID int64, name, purpose string, visibility models.RoomVisibility) (models.ChatRoom, error) {
	room, err := s.requireMember(ctx, roomID, actor.ID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	managed, err := s.canManageRoom(ctx, room, actor.ID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if !managed {
		return models.ChatRoom{}, models.Forbidden("only the project lead or group owner can edit this chat")
	}

	validName, validPurpose, err := roomFields(name, purpose, room.Kind != models.RoomDM)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if room.Kind == models.RoomProject {
		visibility = room.Visibility
	} else if visibility == "" {
		visibility = room.Visibility
	}
	if !models.IsRoomVisibility(visibility) {
		return models.ChatRoom{}, models.Validation("visibility must be private or discoverable")
	}

	if err := s.db.Repo().UpdateChatRoom(ctx, roomID, validName, validPurpose, visibility); err != nil {
		return models.ChatRoom{}, err
	}
	updated, err := s.GetRoom(ctx, actor, roomID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	s.publishRoom(ctx, roomID, "room.updated", updated)
	return updated, nil
}

func (s *ChatService) ArchiveRoom(ctx context.Context, actor models.User, roomID int64) error {
	room, err := s.requireMember(ctx, roomID, actor.ID)
	if err != nil {
		return err
	}
	if room.Kind == models.RoomDM {
		return models.Forbidden("direct messages are closed by leaving, not archiving")
	}
	managed, err := s.canManageRoom(ctx, room, actor.ID)
	if err != nil {
		return err
	}
	if !managed {
		return models.Forbidden("only the project lead or group owner can archive this chat")
	}
	if err := s.db.Repo().ArchiveChatRoom(ctx, roomID, nowUTC()); err != nil {
		return err
	}
	s.publishRoom(ctx, roomID, "room.archived", map[string]int64{"roomId": roomID})
	return nil
}

func (s *ChatService) AddMember(ctx context.Context, actor models.User, roomID, userID int64) (models.ChatRoom, error) {
	room, err := s.requireMember(ctx, roomID, actor.ID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if room.Kind == models.RoomDM {
		return models.ChatRoom{}, models.Forbidden("direct messages cannot add participants")
	}
	managed, err := s.canManageRoom(ctx, room, actor.ID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if !managed {
		return models.ChatRoom{}, models.Forbidden("only the project lead or group owner can add members")
	}
	if err := s.ensureUser(ctx, userID); err != nil {
		return models.ChatRoom{}, err
	}
	if room.Kind == models.RoomProject {
		projectMember, err := s.db.Repo().IsProjectMember(ctx, *room.ProjectID, userID)
		if err != nil {
			return models.ChatRoom{}, err
		}
		if !projectMember {
			return models.ChatRoom{}, models.Validation("project chat members must first join the project")
		}
	}
	if err := s.db.Repo().AddChatMember(ctx, roomID, userID, "member", nowUTC()); err != nil {
		return models.ChatRoom{}, err
	}
	updated, err := s.GetRoom(ctx, actor, roomID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	s.publishRoom(ctx, roomID, "member.added", updated)
	return updated, nil
}

func (s *ChatService) RemoveMember(ctx context.Context, actor models.User, roomID, userID int64) (models.ChatRoom, error) {
	room, err := s.requireMember(ctx, roomID, actor.ID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if room.Kind == models.RoomDM {
		return models.ChatRoom{}, models.Forbidden("direct messages cannot remove participants")
	}
	if userID == actor.ID {
		if err := s.Leave(ctx, actor, roomID); err != nil {
			return models.ChatRoom{}, err
		}
		return s.GetRoomAfterLeave(ctx, room)
	}
	managed, err := s.canManageRoom(ctx, room, actor.ID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if !managed {
		return models.ChatRoom{}, models.Forbidden("only the project lead or group owner can remove members")
	}
	if room.Kind == models.RoomProject && room.ProjectID != nil {
		project, err := s.db.Repo().ProjectByID(ctx, *room.ProjectID)
		if err != nil {
			return models.ChatRoom{}, err
		}
		if project.CreatorID == userID {
			return models.ChatRoom{}, models.Conflict("the project lead cannot be removed from a project chat")
		}
	}
	if err := s.db.Repo().RemoveChatMember(ctx, roomID, userID); err != nil {
		return models.ChatRoom{}, err
	}
	s.publishRoom(ctx, roomID, "member.removed", map[string]int64{"roomId": roomID, "userId": userID})
	s.hub.Publish([]int64{userID}, events.Event{Type: "room.archived", RoomID: roomID, Data: map[string]int64{"roomId": roomID}})
	updated, err := s.GetRoom(ctx, actor, roomID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	return updated, nil
}

func (s *ChatService) Join(ctx context.Context, actor models.User, roomID int64) (models.ChatRoom, error) {
	room, err := s.db.Repo().ChatRoomByID(ctx, roomID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if room.Kind != models.RoomOpen || room.Visibility != models.RoomDiscoverable || room.Status != models.RoomActive {
		return models.ChatRoom{}, models.Forbidden("this chat is not open to join")
	}
	if err := s.db.Repo().AddChatMember(ctx, roomID, actor.ID, "member", nowUTC()); err != nil {
		return models.ChatRoom{}, err
	}
	updated, err := s.GetRoom(ctx, actor, roomID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	s.publishRoom(ctx, roomID, "member.added", map[string]int64{"roomId": roomID, "userId": actor.ID})
	return updated, nil
}

func (s *ChatService) Leave(ctx context.Context, actor models.User, roomID int64) error {
	room, err := s.requireMember(ctx, roomID, actor.ID)
	if err != nil {
		return err
	}
	if room.Kind == models.RoomProject && room.ProjectID != nil {
		project, err := s.db.Repo().ProjectByID(ctx, *room.ProjectID)
		if err != nil {
			return err
		}
		if project.CreatorID == actor.ID {
			return models.Conflict("the project lead cannot leave a project chat")
		}
	}
	if err := s.db.Repo().RemoveChatMember(ctx, roomID, actor.ID); err != nil {
		return err
	}
	s.publishRoom(ctx, roomID, "member.removed", map[string]int64{"roomId": roomID, "userId": actor.ID})
	return nil
}

// GetRoomAfterLeave avoids returning a room the caller no longer belongs to.
func (s *ChatService) GetRoomAfterLeave(_ context.Context, room models.ChatRoom) (models.ChatRoom, error) {
	room.Members = nil
	return room, nil
}

// ---------- messages ----------

func (s *ChatService) SendMessage(ctx context.Context, actor models.User, roomID int64, body string, attachmentIDs []int64) (models.ChatMessage, error) {
	if _, err := s.requireMember(ctx, roomID, actor.ID); err != nil {
		return models.ChatMessage{}, err
	}
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(body) > maxMessageRunes {
		return models.ChatMessage{}, models.Validation("message must be at most %d characters", maxMessageRunes)
	}
	if body == "" && len(attachmentIDs) == 0 {
		return models.ChatMessage{}, models.Validation("a message needs text or an image")
	}
	if len(attachmentIDs) > maxAttachments {
		return models.ChatMessage{}, models.Validation("a message can carry at most %d images", maxAttachments)
	}
	if err := s.validateAttachments(ctx, roomID, actor.ID, attachmentIDs); err != nil {
		return models.ChatMessage{}, err
	}

	now := nowUTC()
	var message models.ChatMessage
	err := s.db.WithTx(ctx, func(repo *transaction.Repo) error {
		created, err := repo.CreateChatMessage(ctx, roomID, actor.ID, body, now)
		if err != nil {
			return err
		}
		message = created
		if len(attachmentIDs) > 0 {
			if err := repo.AttachChatAttachments(ctx, message.ID, roomID, actor.ID, attachmentIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.ChatMessage{}, err
	}

	message, err = s.db.Repo().ChatMessageByID(ctx, message.ID)
	if err != nil {
		return models.ChatMessage{}, err
	}
	s.publishRoom(ctx, roomID, "message.created", message)
	return message, nil
}

func (s *ChatService) EditMessage(ctx context.Context, actor models.User, messageID int64, body string) (models.ChatMessage, error) {
	message, err := s.db.Repo().ChatMessageByID(ctx, messageID)
	if err != nil {
		return models.ChatMessage{}, err
	}
	if message.SenderID != actor.ID {
		return models.ChatMessage{}, models.Forbidden("you can only edit your own messages")
	}
	if message.DeletedAt != nil {
		return models.ChatMessage{}, models.Conflict("a deleted message cannot be edited")
	}
	if _, err := s.requireMember(ctx, message.RoomID, actor.ID); err != nil {
		return models.ChatMessage{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return models.ChatMessage{}, models.Validation("a message cannot be empty")
	}
	if utf8.RuneCountInString(body) > maxMessageRunes {
		return models.ChatMessage{}, models.Validation("message must be at most %d characters", maxMessageRunes)
	}

	updated, err := s.db.Repo().UpdateChatMessage(ctx, messageID, body, nowUTC())
	if err != nil {
		return models.ChatMessage{}, err
	}
	s.publishRoom(ctx, message.RoomID, "message.updated", updated)
	return updated, nil
}

func (s *ChatService) DeleteMessage(ctx context.Context, actor models.User, messageID int64) error {
	message, err := s.db.Repo().ChatMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	room, err := s.requireMember(ctx, message.RoomID, actor.ID)
	if err != nil {
		return err
	}
	if message.SenderID != actor.ID {
		managed, err := s.canManageRoom(ctx, room, actor.ID)
		if err != nil {
			return err
		}
		if !managed {
			return models.Forbidden("you can only delete your own messages")
		}
	}
	deleted, err := s.db.Repo().SoftDeleteChatMessage(ctx, messageID, nowUTC())
	if err != nil {
		return err
	}
	s.publishRoom(ctx, message.RoomID, "message.deleted", deleted)
	return nil
}

func (s *ChatService) MarkRead(ctx context.Context, actor models.User, roomID, messageID int64) error {
	if _, err := s.requireMember(ctx, roomID, actor.ID); err != nil {
		return err
	}
	exists, err := s.db.Repo().MessageExistsInRoom(ctx, roomID, messageID)
	if err != nil {
		return err
	}
	if !exists {
		return models.NotFound("message not found")
	}
	return s.db.Repo().MarkChatRead(ctx, roomID, actor.ID, messageID)
}

// ---------- attachments ----------

func (s *ChatService) SaveAttachment(ctx context.Context, actor models.User, roomID int64, filename, mime string, data []byte, width, height *int) (models.ChatAttachment, error) {
	if _, err := s.requireMember(ctx, roomID, actor.ID); err != nil {
		return models.ChatAttachment{}, err
	}
	key, err := s.store.Save(ctx, filename, data)
	if err != nil {
		return models.ChatAttachment{}, err
	}
	attachment, err := s.db.Repo().CreateChatAttachment(
		ctx, roomID, actor.ID, key, filename, mime, int64(len(data)), width, height, nowUTC(),
	)
	if err != nil {
		_ = s.store.Delete(ctx, key)
		return models.ChatAttachment{}, err
	}
	return attachment, nil
}

func (s *ChatService) Attachment(ctx context.Context, actor models.User, attachmentID int64) (models.ChatAttachment, []byte, error) {
	attachment, err := s.db.Repo().ChatAttachmentByID(ctx, attachmentID)
	if err != nil {
		return models.ChatAttachment{}, nil, err
	}
	if _, err := s.requireMember(ctx, attachment.RoomID, actor.ID); err != nil {
		return models.ChatAttachment{}, nil, err
	}
	data, err := s.store.Read(ctx, attachment.StorageKey)
	if err != nil {
		return models.ChatAttachment{}, nil, err
	}
	return attachment, data, nil
}

func (s *ChatService) validateAttachments(ctx context.Context, roomID, uploaderID int64, ids []int64) error {
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			return models.Validation("the same image cannot be attached twice")
		}
		seen[id] = true

		attachmentRoom, uploader, messageID, err := s.db.Repo().ChatAttachmentUploader(ctx, id)
		if err != nil {
			return err
		}
		if attachmentRoom != roomID {
			return models.Forbidden("an attachment belongs to another chat")
		}
		if uploader != uploaderID {
			return models.Forbidden("you can only attach images you uploaded")
		}
		if messageID != nil {
			return models.Conflict("an attachment was already sent")
		}
	}
	return nil
}

// ---------- requests ----------

func (s *ChatService) Requests(ctx context.Context, actor models.User) ([]models.ChatRequest, error) {
	return s.db.Repo().ChatRequestsForUser(ctx, actor.ID)
}

// ---------- helpers ----------

func (s *ChatService) requireMember(ctx context.Context, roomID, userID int64) (models.ChatRoom, error) {
	isMember, err := s.db.Repo().IsChatMember(ctx, roomID, userID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if !isMember {
		return models.ChatRoom{}, models.Forbidden("you are not a member of this chat")
	}
	room, err := s.db.Repo().ChatRoomByID(ctx, roomID)
	if err != nil {
		return models.ChatRoom{}, err
	}
	if room.Status != models.RoomActive {
		return models.ChatRoom{}, models.Forbidden("this chat has been archived")
	}
	return room, nil
}

func (s *ChatService) canManageRoom(ctx context.Context, room models.ChatRoom, userID int64) (bool, error) {
	switch room.Kind {
	case models.RoomProject:
		if room.ProjectID == nil {
			return false, nil
		}
		project, err := s.db.Repo().ProjectByID(ctx, *room.ProjectID)
		if err != nil {
			return false, err
		}
		return project.CreatorID == userID, nil
	case models.RoomOpen:
		role, err := s.db.Repo().ChatMemberRole(ctx, room.ID, userID)
		if err != nil {
			if isNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return role == "owner", nil
	default:
		return false, nil
	}
}

func (s *ChatService) validateProjectRoomMembers(ctx context.Context, projectID, creatorID int64, ids []int64) ([]int64, error) {
	result := []int64{}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id == creatorID || seen[id] {
			continue
		}
		seen[id] = true
		projectMember, err := s.db.Repo().IsProjectMember(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		if !projectMember {
			return nil, models.Validation("every project chat member must belong to the project")
		}
		result = append(result, id)
	}
	return result, nil
}

func (s *ChatService) uniqueExistingUsers(ctx context.Context, actorID int64, ids []int64) ([]int64, error) {
	result := []int64{}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id == actorID || seen[id] {
			continue
		}
		seen[id] = true
		if err := s.ensureUser(ctx, id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}

func (s *ChatService) ensureUser(ctx context.Context, userID int64) error {
	exists, err := s.db.Repo().UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return models.NotFound("user not found")
	}
	return nil
}

func (s *ChatService) publishRoom(ctx context.Context, roomID int64, eventType string, data any) {
	members, err := s.db.Repo().ChatRoomMemberIDs(ctx, roomID)
	if err != nil {
		return
	}
	s.hub.Publish(members, events.Event{Type: eventType, RoomID: roomID, Data: data})
}

func roomFields(name, purpose string, requireName bool) (string, string, error) {
	name = strings.TrimSpace(name)
	purpose = strings.TrimSpace(purpose)
	if requireName {
		if count := utf8.RuneCountInString(name); count < 3 || count > maxRoomName {
			return "", "", models.Validation("chat name must be between 3 and %d characters", maxRoomName)
		}
	}
	if utf8.RuneCountInString(purpose) > maxPurposeRunes {
		return "", "", models.Validation("chat purpose must be at most %d characters", maxPurposeRunes)
	}
	return name, purpose, nil
}

func dmKey(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("%d:%d", a, b)
}
