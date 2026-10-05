package model

// Task is a node task as returned by GET /nodes/{node}/tasks.
type Task struct {
	UPID       string `json:"upid"`
	Type       string `json:"type"`
	Node       string `json:"node"`
	User       string `json:"user,omitempty"`
	ID         string `json:"id,omitempty"`
	StartTime  int64  `json:"starttime"`
	EndTime    int64  `json:"endtime,omitempty"`
	Status     string `json:"status,omitempty"`
	ExitStatus string `json:"exitstatus,omitempty"`
	PID        int    `json:"pid,omitempty"`
}

// TaskStatus is the status of a task by UPID from
// GET /nodes/{node}/tasks/{upid}/status.
type TaskStatus struct {
	UPID       string `json:"upid"`
	Type       string `json:"type"`
	Node       string `json:"node"`
	User       string `json:"user,omitempty"`
	StartTime  int64  `json:"starttime"`
	EndTime    int64  `json:"endtime,omitempty"`
	Status     string `json:"status,omitempty"`
	ExitStatus string `json:"exitstatus,omitempty"`
	PID        int    `json:"pid,omitempty"`
	Running    bool   `json:"running,omitempty"`
}

// TaskLogEntry is a single line of a task log from
// GET /nodes/{node}/tasks/{upid}/log.
type TaskLogEntry struct {
	LineNumber int    `json:"n"`
	Text       string `json:"t"`
}
