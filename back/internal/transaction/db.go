package transaction

import (
	"time"

	_ "modernc.org/sqlite"

	"database/sql"
	"encoding/json"

	"rudy/internal/models"
)

type Transaction struct {
	db *sql.DB
}

var schema = `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		description TEXT NOT NULL,
		task_type VARCHAR(10) NOT NULL,
		task_state VARCHAR(10) NOT NULL,
		creator TEXT NOT NULL,
		assignee TEXT NOT NULL,
		role TEXT,
		points INTEGER,
		dependencies TEXT,
		acceptance_criteria TEXT,
		created_at TIMESTAMPZ NOT NULL DEFAULT now(),
		completed_at TIMESTAMPZ,
	);

	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name VARCHAR(24) NOT NULL,
		description TEXT NOT NULL,
		tasks TEXT DEFAULT '[]',
		ttl INTEGER
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username VARCHAR(24) NOT NULL,
		role VARCHAR(24) NOT NULL,
		points INTEGER NOT NULL,
	);
`

func Init(path string) (*Transaction, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// test connection
	if err = db.Ping(); err != nil {
		return nil, err
	}

	// perform migration
	if _, err = db.Exec(schema); err != nil {
		return nil, err
	}
	
	return &Transaction{ db: db }, nil
} 

func (t *Transaction) InsertUser(user models.User) error {
	_, err := t.db.Exec(`
		INSERT INTO users (id, username, role, points) VALUES (?, ?, ?, ?)`, user.Id, user.Username, user.Role, user.Points,
	)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transaction) UpdateRole(role models.Role, user models.User) error {
	_, err := t.db.Exec(`
		UPDATE users set role = ? WHERE id = ?`,
		role, user.Id)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transaction) DeleteUser(user models.User) error {
	_, err := t.db.Exec(`
		DELETE FROM users WHERE id = ?`, user.Id)
	if err != nil {
		return err
	}
	return nil
}

func (t *Transaction) AccumulatePoints(user models.User, points int) error {
	row, err := t.db.Query(`
		SELECT points FROM users WHERE id = ?`, user.Id)
	if err != nil {
		return err
	}
	defer row.Close()

	var pointsBefore int
	if row.Next() {
		err = row.Scan(&pointsBefore)
		if err != nil {
			return err
		}
	}

	// increment points then write back to db
	points = points + pointsBefore
	_, err = t.db.Exec(`
		UPDATE users SET points = ? where id = ?`, points, user.Id)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transaction) InsertTask(task models.Task) error {
	creator, err := json.Marshal(task.Creator)
	if err != nil {
		return err
	}
	
	assignee, err := json.Marshal(task.Assignee)
	if  err != nil {
		return err
	}

	dependencies, err := json.Marshal(task.Dependencies)
	if err != nil {
		return err
	}

	a_criteria, err := json.Marshal(task.Dependencies)
	if err != nil {
		return err
	}
	_, err = t.db.Exec(`
		INSERT INTO tasks (id, description, tasktype, taskstate, creator, assignee,
		role, points, dependencies, acceptance_creteria, created_at, completed_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`,
		task.Id, task.Description, task.TaskType, task.TaskState, creator, assignee,
		task.Role, task.Points, dependencies, a_criteria, task.CreatedAt, task.CompleteAt,
	)
	if err != nil {
		return err
	}

	return nil
}


func (t *Transaction) ReassignTask(user models.User) error {
	jsonUser, err := json.Marshal(user)
	if err != nil {
		return err
	}
	json := string(jsonUser)

	_, err = t.db.Exec(`
		UPDATE tasks SET assignee = ?`, json)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transaction) AlterTaskState(task models.Task) error {
	_, err := t.db.Exec(`
		UPDATE tasks SET tasktype = ? WHERE id = ?`, task.TaskType, task.Id)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transaction) InsertProject(project models.Project) error {
	tasks, err := json.Marshal(project.Tasks)
	if err != nil {
		return err
	}
	_, err = t.db.Exec(`
		INSERT INTO projects (id, name, description, tasks, ttl VALUES (?, ?, ?, ?, ?)`,
		project.Id, project.Name, project.Description, tasks, project.Ttl)
	
	if err != nil {
		return err
	}
	return nil
}

func (t *Transaction) DeleteProject(project models.Project) error {
	_, err := t.db.Exec(`
		DELETE FROM projects where id = ?`, project.Id)
	if err != nil {
		return err
	}
	return nil
}

func (t *Transaction) UpdateTasks(project models.Project) error {
	newTasks, err := json.Marshal(project.Tasks)
	if err != nil {
		return err
	}
	tasks := string(newTasks)
	
	_, err = t.db.Exec(`
		UPDATE tasks SET tasks = ? where id = ?`, tasks, project.Id)

	if err != nil {
		return err
	}
	return nil
}

func (t *Transaction) SelectUserTasks(user models.User) ([]models.Task, error) {
	u, err := json.Marshal(user)
	if err != nil {
		return []models.Task{}, err
	}
	rows, err := t.db.Query(`
		SELECT * FROM tasks WHERE assignee = ?`, u)
	if err != nil {
		return []models.Task{}, err
	}
	defer rows.Close()
	
	result := []models.Task{}

	for rows.Next() {
		var (
			id int64
			description string
			tasktype string
			taskstate string
			creator string
			assignee string
			role string
			points int
			dependencies string
			a_criteria string
			created_at time.Time
			completed_at time.Time
		)
		err = rows.Scan(&id, &description, &tasktype, &taskstate, &creator, &assignee, &role, &points, &dependencies, &a_criteria, &created_at, &completed_at)
		if err != nil {
			return []models.Task{}, err
		}

		var (
			ttype models.TaskType
			tstate models.TaskState
			c models.User
			a models.User
			d []models.Task
			ac models.Done
		)
	
		err = json.Unmarshal([]byte(tasktype), &ttype)
		if err != nil {
			return []models.Task{}, err 
		}

		err = json.Unmarshal([]byte(taskstate), &tstate)
		if err != nil {
			return []models.Task{}, err
		}


		err = json.Unmarshal([]byte(creator), &c)
		if err != nil {
			return []models.Task{}, err
		}
		err = json.Unmarshal([]byte(assignee), &a)
		if err != nil {
			return []models.Task{}, err
		}

		err = json.Unmarshal([]byte(dependencies), &d)
		if err != nil {
			return []models.Task{}, err
		}

		err = json.Unmarshal([]byte(a_criteria), &ac)
		if err != nil {
			return []models.Task{}, err
		}
		
		task := models.Task {
			Id: id,
			Description: description,
			TaskType: ttype,
			TaskState: tstate,
			Creator: c,
			Assignee: &a,
			Role: role,
			Points: points,
			Dependencies: d,
			AcceptanceCriteria: ac,
			CreatedAt: created_at,
			CompleteAt: completed_at,
		}
		result = append(result, task)
	}
	if err = rows.Err(); err != nil {
		return []models.Task{}, err
	}
	return result, nil
}