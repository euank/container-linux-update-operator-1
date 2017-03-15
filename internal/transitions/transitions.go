// Package transitions represents available transitions for a node
package transitions

import "github.com/coreos-inc/container-linux-update-operator/internal/constants"

var agentTransitionMap = map[constants.StatusAnnotation]map[constants.StatusAnnotation]struct{}{
	NoUpdateAvailable: {
		UpdateAvailable: struct{}{},
	},
	UpdateAvailable: {},
	UpdateExpected: {
		UpdateInProgress: struct{}{},
	},
	UpdateInProgress: {
		NoUpdateAvailable: struct{}{},
	},
	UpdateDisabled: {},
}

var operatorTransitionMap = map[constants.StatusAnnotation]map[constants.StatusAnnotation]struct{}{
	UpdateAvailable: {
		UpdateExpected: struct{}{},
	},
}
