package plugin

import (
	"log/slog"

	"github.com/sst/sidecar/internal/adapter"
	"github.com/sst/sidecar/internal/event"
)

// Context provides shared resources to plugins during initialization.
type Context struct {
	WorkDir   string
	ConfigDir string
	Adapters  map[string]adapter.Adapter
	EventBus  *event.Dispatcher
	Logger    *slog.Logger
}
