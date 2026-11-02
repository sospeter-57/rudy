package routes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rudy/internal/models"
	"rudy/internal/services"
)

const (
	maxUploadBytes = 8 << 20 // 8 MiB per image
	uploadOverhead = 1 << 20 // multipart framing allowance
)

// ---------- rooms ----------

func (s *server) listChats(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	rooms, err := s.chat.ListRooms(r.Context(), s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ChatRoom{"rooms": rooms})
}

func (s *server) listDiscoverableChats(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	rooms, err := s.chat.Discover(r.Context(), s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ChatRoom{"rooms": rooms})
}

func (s *server) listChatRequests(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	requests, err := s.chat.Requests(r.Context(), s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ChatRequest{"requests": requests})
}

func (s *server) chatUnread(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	total, err := s.chat.UnreadTotal(r.Context(), s.currentUser(r))
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread": total})
}

func (s *server) fetchChat(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	room, err := s.chat.GetRoom(r.Context(), s.currentUser(r), id)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ChatRoom{"room": room})
}

func (s *server) openDM(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	var req dmRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	room, err := s.chat.OpenDM(r.Context(), s.currentUser(r), req.UserID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ChatRoom{"room": room})
}

func (s *server) createOpenChat(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	var req openRoomRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	result, err := s.chat.CreateOpenRoom(r.Context(), s.currentUser(r), services.OpenRoomInput{
		Name:       req.Name,
		Purpose:    req.Purpose,
		Visibility: req.Visibility,
		InviteIDs:  req.InviteIDs,
	})
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *server) updateChat(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var req roomRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	room, err := s.chat.UpdateRoom(r.Context(), s.currentUser(r), id, req.Name, req.Purpose, req.Visibility)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ChatRoom{"room": room})
}

func (s *server) archiveChat(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	if err := s.chat.ArchiveRoom(r.Context(), s.currentUser(r), id); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) joinChat(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	room, err := s.chat.Join(r.Context(), s.currentUser(r), id)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ChatRoom{"room": room})
}

func (s *server) leaveChat(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	if err := s.chat.Leave(r.Context(), s.currentUser(r), id); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) addChatMember(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var req memberRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	room, err := s.chat.AddMember(r.Context(), s.currentUser(r), id, req.UserID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ChatRoom{"room": room})
}

func (s *server) removeChatMember(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathInt(w, r, "userId")
	if !ok {
		return
	}
	room, err := s.chat.RemoveMember(r.Context(), s.currentUser(r), id, userID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ChatRoom{"room": room})
}

// ---------- messages ----------

func (s *server) listChatMessages(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}

	query := r.URL.Query()
	var before *int64
	if raw := strings.TrimSpace(query.Get("before")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			writeError(w, http.StatusBadRequest, "before must be a positive integer")
			return
		}
		before = &value
	}
	limit := 50
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 || value > 100 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = value
	}

	messages, err := s.chat.Messages(r.Context(), s.currentUser(r), id, before, limit)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ChatMessage{"messages": messages})
}

func (s *server) sendChatMessage(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var req messageRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	message, err := s.chat.SendMessage(r.Context(), s.currentUser(r), id, req.Body, req.AttachmentIDs)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]models.ChatMessage{"message": message})
}

func (s *server) editChatMessage(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var req messageRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	message, err := s.chat.EditMessage(r.Context(), s.currentUser(r), id, req.Body)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ChatMessage{"message": message})
}

func (s *server) deleteChatMessage(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	if err := s.chat.DeleteMessage(r.Context(), s.currentUser(r), id); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) markChatRead(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var req readRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.chat.MarkRead(r.Context(), s.currentUser(r), id, req.MessageID); err != nil {
		s.respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- attachments ----------

func (s *server) uploadChatAttachment(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	roomID, ok := pathInt(w, r, "id")
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+uploadOverhead)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "image is too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid multipart upload")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "a file field is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read the upload")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "the image is empty")
		return
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "image is too large")
		return
	}

	mimeType := http.DetectContentType(data)
	if !isImageMime(mimeType) {
		writeError(w, http.StatusUnsupportedMediaType, "only PNG, JPEG, GIF or WebP images are allowed")
		return
	}

	filename := sanitizeFilename(header.Filename, mimeType)
	width, height := imageDimensions(data)

	attachment, err := s.chat.SaveAttachment(
		r.Context(), s.currentUser(r), roomID, filename, mimeType, data, width, height,
	)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]models.ChatAttachment{"attachment": attachment})
}

func (s *server) serveChatAttachment(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	attachment, data, err := s.chat.Attachment(r.Context(), s.currentUser(r), id)
	if err != nil {
		s.respondError(w, err)
		return
	}

	w.Header().Set("Content-Type", attachment.Mime)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": attachment.Filename}))
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func isImageMime(value string) bool {
	switch value {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func sanitizeFilename(name, mimeType string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		switch mimeType {
		case "image/png":
			return "image.png"
		case "image/jpeg":
			return "image.jpg"
		case "image/gif":
			return "image.gif"
		case "image/webp":
			return "image.webp"
		default:
			return "image"
		}
	}
	if len(name) > 120 {
		name = name[len(name)-120:]
	}
	return name
}

func imageDimensions(data []byte) (*int, *int) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return nil, nil
	}
	return &config.Width, &config.Height
}

// ---------- server-sent events ----------

// chatEvents streams room activity for the signed-in user. Native EventSource
// cannot attach a bearer header, so the client reads this response as a fetch
// stream instead.
func (s *server) chatEvents(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	user := s.currentUser(r)
	eventsChannel, unsubscribe := s.chat.Hub().Subscribe(user.ID)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// The server has a 15 second write timeout; streams must outlive it.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-eventsChannel:
			if !open {
				return
			}
			payload, err := json.Marshal(event.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

// ---------- project rooms and membership ----------

func (s *server) listProjectChats(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	rooms, err := s.chat.ProjectRooms(r.Context(), s.currentUser(r), id)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ChatRoom{"rooms": rooms})
}

func (s *server) createProjectChat(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusNotImplemented, "chat service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var req projectRoomRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	room, err := s.chat.CreateProjectRoom(r.Context(), s.currentUser(r), id, services.ProjectRoomInput{
		Name:      req.Name,
		Purpose:   req.Purpose,
		MemberIDs: req.MemberIDs,
	})
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]models.ChatRoom{"room": room})
}

func (s *server) listProjectMembers(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	members, err := s.projects.Members(r.Context(), id)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ProjectMember{"members": members})
}

func (s *server) addProjectMember(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var req memberRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	members, err := s.projects.AddMember(r.Context(), id, s.currentUser(r).ID, req.UserID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ProjectMember{"members": members})
}

func (s *server) removeProjectMember(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	userID, ok := pathInt(w, r, "userId")
	if !ok {
		return
	}
	members, err := s.projects.RemoveMember(r.Context(), id, s.currentUser(r).ID, userID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ProjectMember{"members": members})
}

func (s *server) requestProjectJoin(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	request, err := s.projects.RequestJoin(r.Context(), id, s.currentUser(r).ID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]models.ProjectJoinRequest{"request": request})
}

func (s *server) listProjectJoinRequests(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	requests, err := s.projects.JoinRequests(r.Context(), id, s.currentUser(r).ID)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]models.ProjectJoinRequest{"requests": requests})
}

func (s *server) approveProjectJoinRequest(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	requestID, ok := pathInt(w, r, "requestId")
	if !ok {
		return
	}
	request, err := s.projects.ReviewJoinRequest(r.Context(), id, requestID, s.currentUser(r).ID, true)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ProjectJoinRequest{"request": request})
}

func (s *server) rejectProjectJoinRequest(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeError(w, http.StatusNotImplemented, "project service not configured")
		return
	}
	id, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	requestID, ok := pathInt(w, r, "requestId")
	if !ok {
		return
	}
	request, err := s.projects.ReviewJoinRequest(r.Context(), id, requestID, s.currentUser(r).ID, false)
	if err != nil {
		s.respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]models.ProjectJoinRequest{"request": request})
}
