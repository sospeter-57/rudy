package routes

import "rudy/internal/models"

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type googleAuthRequest struct {
	Credential string       `json:"credential"`
	Role       *models.Role `json:"role"`
}

type registerRequest struct {
	Username string      `json:"username"`
	Password string      `json:"password"`
	Role     models.Role `json:"role"`
}

type projectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TTL         int    `json:"ttl"`
}

type taskRequest struct {
	Description  string          `json:"description"`
	TaskType     models.TaskType `json:"taskType"`
	Role         models.Role     `json:"role"`
	Points       int             `json:"points"`
	Dependencies []int64         `json:"dependencies"`
}

type assignRequest struct {
	AssigneeID int64 `json:"assignee_id"`
}

type reviewerRequest struct {
	ReviewerID int64 `json:"reviewer_id"`
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

type codeRequest struct {
	Code string `json:"code"`
}

type testsRequest struct {
	Tests string `json:"tests"`
}

type documentationRequest struct {
	Documentation string `json:"documentation"`
}

type reviewRequest struct {
	Decision models.ReviewDecision `json:"decision"`
	Comment  string                `json:"comment"`
}

type dmRequest struct {
	UserID int64 `json:"userId"`
}

type roomRequest struct {
	Name       string                `json:"name"`
	Purpose    string                `json:"purpose"`
	Visibility models.RoomVisibility `json:"visibility,omitempty"`
}

type openRoomRequest struct {
	Name       string                `json:"name"`
	Purpose    string                `json:"purpose"`
	Visibility models.RoomVisibility `json:"visibility,omitempty"`
	InviteIDs  []int64               `json:"inviteIds,omitempty"`
}

type projectRoomRequest struct {
	Name      string  `json:"name"`
	Purpose   string  `json:"purpose"`
	MemberIDs []int64 `json:"memberIds,omitempty"`
}

type messageRequest struct {
	Body          string  `json:"body"`
	AttachmentIDs []int64 `json:"attachmentIds,omitempty"`
}

type readRequest struct {
	MessageID int64 `json:"messageId"`
}

type memberRequest struct {
	UserID int64 `json:"userId"`
}

type preferencesRequest struct {
	ThemeMode models.ThemeMode `json:"themeMode"`
	Accent    string           `json:"accent"`
	BlendA    string           `json:"blendA"`
	BlendB    string           `json:"blendB"`
}

func (r preferencesRequest) Preferences() models.Preferences {
	return models.Preferences{
		ThemeMode: r.ThemeMode,
		Accent:    r.Accent,
		BlendA:    r.BlendA,
		BlendB:    r.BlendB,
	}
}
