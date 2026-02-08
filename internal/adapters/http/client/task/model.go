package task

import "time"

type Task struct {
	Name      string
	ExecuteAt time.Time
}
