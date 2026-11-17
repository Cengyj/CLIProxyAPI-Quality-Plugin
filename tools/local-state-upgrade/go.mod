module github.com/cliproxyapi-plugins/local-state-upgrade

go 1.26.10

require (
	github.com/cliproxyapi-plugins/model-quality v0.0.0
	github.com/google/uuid v1.6.0
	go.etcd.io/bbolt v1.5.0
)

require (
	github.com/MoeForge/moego v0.1.0-alpha.1 // indirect
	golang.org/x/sys v0.48.0 // indirect
)

replace github.com/cliproxyapi-plugins/model-quality => ../../plugin

replace github.com/router-for-me/CLIProxyAPI/v8 => ../../workspaces/CLIProxyAPI-official

replace github.com/MoeForge/moego v0.1.0-alpha.1 => github.com/router-for-me/moego v0.1.2
