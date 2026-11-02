package models

import "time"

// ProjectMember is an explicit membership row. Project membership currently
// gates chat access only; projects and tasks stay visible to every signed-in
// user.
type ProjectMember struct {
	UserID      int64     `json:"userId"`
	Username    string    `json:"username"`
	Role        Role      `json:"role"`
	ProjectRole string    `json:"projectRole"`
	AddedBy     *int64    `json:"addedBy,omitempty"`
	JoinedAt    time.Time `json:"joinedAt"`
}

type RoomKind string

const (
	RoomDM      RoomKind = "dm"
	RoomProject RoomKind = "project"
	RoomOpen    RoomKind = "open"
)

type RoomVisibility string

const (
	RoomPrivate      RoomVisibility = "private"
	RoomDiscoverable RoomVisibility = "discoverable"
)

type RoomStatus string

const (
	RoomActive   RoomStatus = "active"
	RoomArchived RoomStatus = "archived"
)

func IsRoomVisibility(value RoomVisibility) bool {
	return value == RoomPrivate || value == RoomDiscoverable
}

type ChatRoom struct {
	ID          int64          `json:"id"`
	Kind        RoomKind       `json:"kind"`
	Name        string         `json:"name,omitempty"`
	Purpose     string         `json:"purpose,omitempty"`
	ProjectID   *int64         `json:"projectId,omitempty"`
	ProjectName string         `json:"projectName,omitempty"`
	CreatedBy   int64          `json:"createdBy"`
	Visibility  RoomVisibility `json:"visibility"`
	Status      RoomStatus     `json:"status"`
	CreatedAt   time.Time      `json:"createdAt"`
	ArchivedAt  *time.Time     `json:"archivedAt,omitempty"`
	Members     []ChatMember   `json:"members,omitempty"`
	LastMessage *ChatMessage   `json:"lastMessage,omitempty"`
	UnreadCount int            `json:"unreadCount"`
}

type ChatMember struct {
	UserID   int64     `json:"userId"`
	Username string    `json:"username"`
	Role     Role      `json:"role"`
	RoomRole string    `json:"roomRole"`
	JoinedAt time.Time `json:"joinedAt"`
}

type ChatMessage struct {
	ID          int64            `json:"id"`
	RoomID      int64            `json:"roomId"`
	SenderID    int64            `json:"senderId"`
	SenderName  string           `json:"senderName"`
	Body        string           `json:"body"`
	Attachments []ChatAttachment `json:"attachments"`
	CreatedAt   time.Time        `json:"createdAt"`
	EditedAt    *time.Time       `json:"editedAt,omitempty"`
	DeletedAt   *time.Time       `json:"deletedAt,omitempty"`
}

type ChatAttachment struct {
	ID         int64     `json:"id"`
	RoomID     int64     `json:"roomId"`
	UploaderID int64     `json:"-"`
	MessageID  *int64    `json:"messageId,omitempty"`
	StorageKey string    `json:"-"`
	Filename   string    `json:"filename"`
	Mime       string    `json:"mime"`
	SizeBytes  int64     `json:"sizeBytes"`
	Width      *int      `json:"width,omitempty"`
	Height     *int      `json:"height,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type ChatRequest struct {
	ID            int64          `json:"id"`
	RequesterID   int64          `json:"requesterId"`
	RequesterName string         `json:"requesterName"`
	Name          string         `json:"name"`
	Purpose       string         `json:"purpose"`
	Visibility    RoomVisibility `json:"visibility"`
	Status        string         `json:"status"`
	RoomID        *int64         `json:"roomId,omitempty"`
	Invites       []int64        `json:"invites"`
	CreatedAt     time.Time      `json:"createdAt"`
	ReviewedAt    *time.Time     `json:"reviewedAt,omitempty"`
}
