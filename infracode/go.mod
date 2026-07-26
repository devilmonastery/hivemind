module github.com/devilmonastery/hivemind/infracode

go 1.26.4

require (
	github.com/devilmonastery/env-k8s-home v0.0.0
	github.com/devilmonastery/infracode v0.0.0
)

require gopkg.in/yaml.v3 v3.0.1 // indirect

replace github.com/devilmonastery/env-k8s-home => ../../env-k8s-home

replace github.com/devilmonastery/infracode => ../../infracode
