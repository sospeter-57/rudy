package models

import (
	"errors"
	"time"
)
type Task struct {
	Id int64					`json:"id"`
	Description string			`json:"description"`
	TaskType TaskType			`json:"task_type"`
	TaskState TaskState			`json:"task_state"`
	Creator User				`json:"creator"`
	Assignee *User				`json:"assignee"`
	Role string					`json:"role"` // to make clear what roles someone has been working on for analysis
	Points int					`json:"points"`
	Dependencies []Task			`json:"dependencies"`
	AcceptanceCriteria Done		`json:"acceptance_criteria"`
	CreatedAt time.Time			`json:"created_at"`
	CompleteAt time.Time		`json:"completed_at"`
}

type TaskType string;

const (
	Feature TaskType = "feature"
	Bug TaskType = "bug"
	Fix TaskType = "fix"
	Research TaskType = "research"
)

type TaskState string;

const (
	Pending TaskState = "pending"
	Auditing TaskState = "auditing"
	Completed TaskState = "completed"
)

type Done struct {
	CodeImplementation string
	Tests string
	Reviewers []User
	Documentation string
}

type TaskMethods interface {
	assign(user *User, task Task)
	audit()
	complete()
	ttl()
	noteCode(string)
	noteTests(string)
	noteReviewer(User)
	noteDocumentation()
	ifDone() bool
}


// this function checks the whether the user
// has permission to crate a project, returns err if not authorized
func CreateTask(description string, tasktype TaskType, creator, assignee User, role string, points int) (Task, error) {
	newTask := Task {
		Id: 0,
		Description: description,
		TaskType: tasktype,
		TaskState: Pending,
		Creator: creator,
		Assignee: &assignee,
		Role: role,
		Points: points,
	}

	return newTask, nil
}

func (t *Task) assign(user *User) error {
	if t.Assignee == user {
		return errors.New(string(AlreadyAssigned))
	}

	t.Assignee = user
	return nil
}

func (t *Task) audit() error {
	if t.TaskState == Auditing {
		return errors.New(string(AlreadyAudited))
	}

	if time.Now().After(t.CompleteAt) {
		return errors.New(string(DurationAlreadyElapsed))
	}

	t.TaskState = Auditing
	return nil
}

func (t *Task) complete() error {
	if t.TaskState == Completed {
		return errors.New(string(AlreadyCompleted))
	}

	t.TaskState = Completed
	return nil
}

func (t *Task) ttl() float64 {
	return t.CompleteAt.Sub(t.CreatedAt).Minutes()
}

func (t *Task) noteCode(code string) {
	t.AcceptanceCriteria.CodeImplementation = code
}

func (t *Task) noteTests(tests string) {
	t.AcceptanceCriteria.Tests = tests
}

func (t *Task) noteReviewer(reviewer User) {
	t.AcceptanceCriteria.Reviewers = append(t.AcceptanceCriteria.Reviewers, reviewer)
}

func (t *Task) noteDocumentation(doc string) {
	t.AcceptanceCriteria.Documentation = doc
}

func (t *Task) ifDone() bool {
	if t.AcceptanceCriteria.CodeImplementation == "" {
		return false
	} else if t.AcceptanceCriteria.Tests == "" {
		return false
	} else if t.AcceptanceCriteria.Documentation == "" {
		return false
	} else if t.AcceptanceCriteria.Reviewers == nil {
		return false
	} else {
		return true
	}
}

// func (t *Task) advance() {
// 	switch t.TaskState {
// 		case Pending:
// 			t.TaskState = Auditing
// 		case Auditing:
// 			t.TaskState = Completed
// 		default:
// 			t.TaskState = Pending
// 	}
// }
