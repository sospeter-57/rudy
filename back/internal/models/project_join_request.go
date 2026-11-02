package models

import "time"

type ProjectJoinRequestStatus string

const (
	ProjectJoinRequestPending  ProjectJoinRequestStatus = "pending"
	ProjectJoinRequestApproved ProjectJoinRequestStatus = "approved"
	ProjectJoinRequestRejected ProjectJoinRequestStatus = "rejected"
)

type ProjectJoinRequest struct {
	ID           int64                   `json:"id"`
	ProjectID    int64                   `json:"projectId"`
	RequesterID  int64                   `json:"requesterId"`
	RequesterName string                 `json:"requesterName"`
	Status       ProjectJoinRequestStatus `json:"status"`
	CreatedAt    time.Time               `json:"createdAt"`
	ReviewedBy   *int64                  `json:"reviewedBy,omitempty"`
	ReviewedAt   *time.Time              `json:"reviewedAt,omitempty"`
}