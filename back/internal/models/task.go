package models

import (
	"strings"
	"time"
)

type TaskType string

const (
	Feature  TaskType = "feature"
	Bug      TaskType = "bug"
	Fix      TaskType = "fix"
	Research TaskType = "research"
)

func IsTaskType(value TaskType) bool {
	switch value {
	case Feature, Bug, Fix, Research:
		return true
	}
	return false
}

type TaskState string

const (
	Pending   TaskState = "pending"
	Auditing  TaskState = "auditing"
	Completed TaskState = "completed"
)

type ApprovalState string

const (
	Proposed         ApprovalState = "proposed"
	Approved         ApprovalState = "approved"
	ChangesRequested ApprovalState = "changes_requested"
)

type ReviewDecision string

const (
	ReviewPending          ReviewDecision = "pending"
	ReviewApproved         ReviewDecision = "approved"
	ReviewChangesRequested ReviewDecision = "changes_requested"
)

type Review struct {
	UserID    int64          `json:"userId"`
	Decision  ReviewDecision `json:"decision"`
	Comment   string         `json:"comment,omitempty"`
	DecidedAt *time.Time     `json:"decidedAt,omitempty"`
}

type Acceptance struct {
	Code          string   `json:"code"`
	Tests         string   `json:"tests"`
	Documentation string   `json:"documentation"`
	Reviewers     []Review `json:"reviewers"`
}

type Task struct {
	ID              int64         `json:"id"`
	ProjectID       int64         `json:"projectId"`
	Description     string        `json:"description"`
	TaskType        TaskType      `json:"taskType"`
	TaskState       TaskState     `json:"taskState"`
	ApprovalState   ApprovalState `json:"approvalState"`
	CreatorID       int64         `json:"creatorId"`
	AssigneeID      *int64        `json:"assigneeId"`
	Role            Role          `json:"role"`
	Points          int           `json:"points"`
	Dependencies    []int64       `json:"dependencies"`
	Acceptance      Acceptance    `json:"acceptance"`
	RejectionReason string        `json:"rejectionReason,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	CompletedAt     *time.Time    `json:"completedAt,omitempty"`
}

func (t Task) GatesComplete() bool {
	return strings.TrimSpace(t.Acceptance.Code) != "" &&
		strings.TrimSpace(t.Acceptance.Tests) != "" &&
		strings.TrimSpace(t.Acceptance.Documentation) != ""
}

func (t Task) AllReviewersApproved() bool {
	for _, review := range t.Acceptance.Reviewers {
		if review.Decision != ReviewApproved {
			return false
		}
	}
	return true
}

func ReviewBonus(taskPoints int) int {
	bonus := int(float64(taskPoints)*0.15 + 0.5)
	if bonus < 2 {
		return 2
	}
	return bonus
}

type CompletionResult struct {
	Task   Task          `json:"task"`
	Awards []LedgerEntry `json:"awards"`
}
