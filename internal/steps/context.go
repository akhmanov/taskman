package steps

type Context struct {
	RuntimeRoot   string            `json:"runtime_root,omitempty"`
	ProjectSlug   string            `json:"project_slug,omitempty"`
	ProjectTraits map[string]string `json:"project_traits,omitempty"`
	TaskSlug      string            `json:"task_slug,omitempty"`
	TaskRepo      string            `json:"task_repo,omitempty"`
	TaskTraits    map[string]string `json:"task_traits,omitempty"`
	RepoRoot      string            `json:"repo_root,omitempty"`
	BranchName    string            `json:"branch_name,omitempty"`
	WorktreePath  string            `json:"worktree_path,omitempty"`
	TaskDir       string            `json:"task_dir,omitempty"`
	ArtifactsDir  string            `json:"artifacts_dir,omitempty"`
	Phase         string            `json:"phase,omitempty"`
}
