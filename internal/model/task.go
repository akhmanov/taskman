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
	Version   int               `yaml:"version"`
	Slug      string            `yaml:"slug"`
	Project   string            `yaml:"project"`
	Repo      string            `yaml:"repo"`
	Status    TaskStatus        `yaml:"status"`
	Labels    []string          `yaml:"labels,omitempty"`
	Traits    map[string]string `yaml:"traits,omitempty"`
	CreatedAt string            `yaml:"created_at,omitempty"`
	UpdatedAt string            `yaml:"updated_at,omitempty"`
	Blocker   *string           `yaml:"blocker,omitempty"`
	Handoff   bool              `yaml:"handoff"`
	Session   TaskSessionState  `yaml:"session"`
	MR        TaskMRState       `yaml:"mr"`
	Worktree  TaskWorktreeState `yaml:"worktree"`
	LastOp    OperationState    `yaml:"last_op,omitempty"`
}

type TaskSessionState struct {
	Active        string  `yaml:"active,omitempty"`
	LastCompleted *string `yaml:"last_completed,omitempty"`
}

type TaskMRState struct {
	Status MRStatus `yaml:"status"`
	Reason *string  `yaml:"reason,omitempty"`
}

type TaskWorktreeState struct {
	Status WorktreeStatus `yaml:"status"`
}
