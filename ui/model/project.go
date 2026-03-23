package model

const (
	ProjectModeOpen  EventType = "ProjectModeOpen"
	ProjectModeClose EventType = "ProjectModeClose"
)

type ProjectStatusView struct {
	Name      string
	Root      string
	Branch    string
	Summary   string
	Dirty     bool
	Modified  int
	Staged    int
	Untracked int
	Ahead     int
	Behind    int
	Changed   int
	Docs      int
	Code      int
	Tests     int
}

type ProjectViewState struct {
	Active bool
	Status ProjectStatusView
}
