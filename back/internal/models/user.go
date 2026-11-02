package models

import "time"

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email,omitempty"`
	Role      Role      `json:"role"`
	Points    int       `json:"points"`
	CreatedAt time.Time `json:"createdAt"`
}

type Role string

const (
	ProjectManager    Role = "Project Manager"
	DatabaseEngineer  Role = "Database Engineer"
	FrontendEngineer  Role = "Frontend Engineer"
	BackendEngineer   Role = "Backend Engineer"
	FullstackEngineer Role = "Fullstack Engineer"
	QualityAssuranceE Role = "QA Engineer"
	SoftwareEngineer  Role = "Software Engineer"
	DataScientist     Role = "Data Scientist"
	UIUXDesigner      Role = "UIUX Designer"
)

var Roles = []Role{
	ProjectManager,
	DatabaseEngineer,
	FrontendEngineer,
	BackendEngineer,
	FullstackEngineer,
	QualityAssuranceE,
	SoftwareEngineer,
	DataScientist,
	UIUXDesigner,
}

func IsRole(role Role) bool {
	for _, candidate := range Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

type AuthResult struct {
	User      User      `json:"user"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type LeaderboardRow struct {
	User           User `json:"user"`
	Rank           int  `json:"rank"`
	CompletedTasks int  `json:"completedTasks"`
	Reviews        int  `json:"reviews"`
}
