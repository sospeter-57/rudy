package models

import "time"

type LedgerReason string

const (
	ReasonTaskCompleted     LedgerReason = "task_completed"
	ReasonReviewBonus       LedgerReason = "review_bonus"
	ReasonAdjustment        LedgerReason = "adjustment"
	ReasonLeadStartingBalance LedgerReason = "lead_starting_balance"
)

type LedgerEntry struct {
	ID        int64        `json:"id"`
	UserID    int64        `json:"userId"`
	TaskID    *int64       `json:"taskId"`
	ProjectID *int64       `json:"projectId"`
	Reason    LedgerReason `json:"reason"`
	Points    int          `json:"points"`
	Note      string       `json:"note"`
	CreatedAt time.Time    `json:"createdAt"`
}
