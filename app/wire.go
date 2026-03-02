package main

import (
	"sync"

	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/ui/model"
)

const Version = "ms-cli v0.1.0"

type SessionModel struct {
	Provider string `yaml:"provider"`
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint,omitempty"`
}

// Application is the top-level composition container.
type Application struct {
	Engine       *loop.Engine
	Permission   *loop.PermissionManager
	EventCh      chan model.Event
	Demo         bool
	WorkDir      string
	RepoURL      string
	Config       Config
	SessionModel SessionModel
	SessionPath  string
	SessionState PersistentState

	taskMu       sync.Mutex
	nextTaskID   int64
	activeTask   int64
	activeCancel func()
}
