package config

// AppMode represents the running mode of the application.
type AppMode int

const (
	ModeCapture AppMode = iota
	ModeServer
)

func (m AppMode) String() string {
	switch m {
	case ModeCapture:
		return "capture"
	case ModeServer:
		return "server"
	default:
		return "unknown"
	}
}
