package quality

// ManagementRoute is the native wire declaration of an exact authenticated plugin route.
type ManagementRoute struct {
	Method string
	Path   string
}

func ManagementRoutes() []ManagementRoute {
	return []ManagementRoute{
		{"GET", "/presets"}, {"POST", "/presets"}, {"DELETE", "/presets"},
		{"GET", "/targets"}, {"GET", "/questions"},
		{"GET", "/runs"}, {"GET", "/runs/detail"}, {"POST", "/runs"}, {"POST", "/runs/cancel"}, {"POST", "/runs/review"},
		{"GET", "/rules"}, {"POST", "/rules"}, {"PUT", "/rules"}, {"DELETE", "/rules"}, {"POST", "/rules/run"}, {"POST", "/rules/pause"},
		{"GET", "/metrics"}, {"GET", "/history"},
		{"GET", "/actions"}, {"POST", "/actions/preview"}, {"POST", "/actions/release"},
		{"GET", "/settings"}, {"PUT", "/settings"},
	}
}
