package models

type StatsTotals struct {
	Users             int     `json:"users"`
	Builders          int     `json:"builders"`
	Projects          int     `json:"projects"`
	Tasks             int     `json:"tasks"`
	CompletedTasks    int     `json:"completedTasks"`
	CompletionRate    float64 `json:"completionRate"`
	PointsMinted      int     `json:"pointsMinted"`
	ReviewsApproved   int     `json:"reviewsApproved"`
	ReviewsDecided    int     `json:"reviewsDecided"`
	ReviewApprovalPct float64 `json:"reviewApprovalPct"`
	AvgCycleHours     float64 `json:"avgCycleHours"`
}

type ProjectStats struct {
	ProjectID     int64   `json:"projectId"`
	Name          string  `json:"name"`
	TotalTasks    int     `json:"totalTasks"`
	Proposed      int     `json:"proposed"`
	Pending       int     `json:"pending"`
	Auditing      int     `json:"auditing"`
	Completed     int     `json:"completed"`
	OpenPoints    int     `json:"openPoints"`
	PaidPoints    int     `json:"paidPoints"`
	CompletionPct float64 `json:"completionPct"`
}

type UserStats struct {
	User           User `json:"user"`
	CompletedTasks int  `json:"completedTasks"`
	Reviews        int  `json:"reviews"`
}

type TrendPoint struct {
	Period       string `json:"period"`
	Proposals    int    `json:"proposals"`
	Completions  int    `json:"completions"`
	Reviews      int    `json:"reviews"`
	PointsMinted int    `json:"pointsMinted"`
}

type ReasonTotal struct {
	Reason string `json:"reason"`
	Points int    `json:"points"`
	Count  int    `json:"count"`
}

type Stats struct {
	Range                string         `json:"range"`
	Totals               StatsTotals    `json:"totals"`
	TasksByState         map[string]int `json:"tasksByState"`
	TasksByApprovalState map[string]int `json:"tasksByApprovalState"`
	TasksByType          map[string]int `json:"tasksByType"`
	RoleDistribution     map[string]int `json:"roleDistribution"`
	Projects             []ProjectStats `json:"projects"`
	Users                []UserStats    `json:"users"`
	Trends               []TrendPoint   `json:"trends"`
	LedgerByReason       []ReasonTotal  `json:"ledgerByReason"`
}
