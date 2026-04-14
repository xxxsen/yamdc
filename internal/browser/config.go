package browser

type Config struct {
	// RemoteURL is the CDP remote debugging address of an external browser.
	// When set, the navigator connects to this browser instead of launching
	// its own headless instance. Accepts formats like "host:9222",
	// "ws://host:9222", "http://host:9222".
	RemoteURL string `json:"remote_url" yaml:"remote_url"`

	DataDir string `json:"-" yaml:"-"`
	Proxy   string `json:"-" yaml:"-"`
}
