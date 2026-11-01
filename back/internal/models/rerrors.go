package models

type RudyError string

func (e RudyError) Error() string {
	return string(e)
}

const (
	// generic errors
	NotFound           RudyError = "not found"
	InvalidCredentials RudyError = "invalid credentials"
	DuplicateUsername  RudyError = "username already taken"
	InvalidInput       RudyError = "invalid input"

	// task state related errors
	AlreadyCompleted RudyError = "task already completed"
	AlreadyAudited   RudyError = "task already audited"

	// task assignment related errors
	AlreadyAssigned RudyError = "task already assigned"

	// errors related to task completion
	DurationAlreadyElapsed RudyError = "duration already elapsed"
)
