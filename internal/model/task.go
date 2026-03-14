package model

type TaskStatus string

const (
	TaskStatusTodo      TaskStatus = "todo"
	TaskStatusActive    TaskStatus = "active"
	TaskStatusBlocked   TaskStatus = "blocked"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type MRStatus string

const (
	MRStatusMissing   MRStatus = "missing"
	MRStatusDraft     MRStatus = "draft"
	MRStatusReady     MRStatus = "ready"
	MRStatusNotNeeded MRStatus = "not-needed"
)

type WorktreeStatus string

const (
	WorktreeStatusMissing WorktreeStatus = "missing"
	WorktreeStatusPresent WorktreeStatus = "present"
	WorktreeStatusCleaned WorktreeStatus = "cleaned"
	WorktreeStatusSkipped WorktreeStatus = "skipped"
)

type TaskState struct {
	Version   int               `json:"version" yaml:"version"`
	Slug      string            `json:"slug" yaml:"slug"`
	Project   string            `json:"project" yaml:"project"`
	Repo      string            `json:"repo" yaml:"repo"`
	Status    TaskStatus        `json:"status" yaml:"status"`
	Labels    []string          `json:"labels,omitempty" yaml:"labels,omitempty"`
	Traits    map[string]string `json:"traits,omitempty" yaml:"traits,omitempty"`
	CreatedAt string            `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt string            `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Blocker   *string           `json:"blocker,omitempty" yaml:"blocker,omitempty"`
	Handoff   bool              `json:"handoff" yaml:"handoff"`
	Session   TaskSessionState  `json:"session" yaml:"session"`
	MR        TaskMRState       `json:"mr" yaml:"mr"`
	Worktree  TaskWorktreeState `json:"worktree" yaml:"worktree"`
	LastOp    OperationState    `json:"last_op,omitempty" yaml:"last_op,omitempty"`
}

type TaskSessionState struct {
	Active        string  `json:"active,omitempty" yaml:"active,omitempty"`
	LastCompleted *string `json:"last_completed,omitempty" yaml:"last_completed,omitempty"`
}

type TaskMRState struct {
	Status MRStatus `json:"status" yaml:"status"`
	Reason *string  `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type TaskWorktreeState struct {
	Status WorktreeStatus `json:"status" yaml:"status"`
}
