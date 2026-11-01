package models

type Project struct {
	Id int64
	Name string
	Description string
	Tasks []Task
	Ttl int64 			// time to live (in minutes(my choice))
}

func CreateProject(name, description string, tasks []Task) (Project, error) {
	newProject := Project {
		Id: 1,
		Name: name,
		Description: description,
		Tasks: tasks,
		Ttl: 1000,
	}

	return newProject, nil
}

// func 