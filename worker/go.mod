module goonhub-worker

go 1.24.1

require (
	github.com/kardianos/service v1.3.0
	golang.org/x/sys v0.40.0
	goonhub v0.0.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v3 v3.0.1
)

replace goonhub => ../
