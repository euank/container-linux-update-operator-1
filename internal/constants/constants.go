// Package constants has Kubernetes label and annotation constants shared by
// the update-agent and update-operator.
package constants

// StatusAnnotation is the type for all valid values of the
// AnnotationUpdateStatus annotation
type StatusAnnotation string

const (
	// Prefix used by all label and annotation keys.
	Prefix = "container-linux-update.v1.coreos.com/"

	// AnnotationUpdateStatus is the current status of the node. The value of
	// this annotation will determine what actions will be taken.
	// This annotation encodes a finite state machine. The states and transitions
	// are specified below.
	//
	// Note: as a reminder for the reader, annotation updates can be safely done
	// concurrently. See
	// https://github.com/kubernetes/community/blob/945936c12a401f09fb19091ebe9a0b4016af6a08/contributors/devel/api-conventions.md#concurrency-control-and-consistency
	AnnotationUpdateStatus = Prefix + "update-status"

	// NoUpdateAvailable is the initial state of a a node. It indicates that
	// `update-engine` has not messaged that an update is available yet.
	// This state will be set by the node-agent.
	NoUpdateAvailable StatusAnnotation = "no-update-available"

	// UpdateAvailable is the state of a node which indicates an update has been
	// downloaded and may be applied.
	// This state will be set by the node-agent.
	UpdateAvailable = "update-available"

	// UpdateExpected is the state of a node which indicates the available update should be applied.
	// This state will be set by the update-operator.
	UpdateExpected = "update-expected"

	// UpdateInProgress is the state of a node which indicates the requested update is underway.
	// This state wil be set by the node agent.
	UpdateInProgress = "update-in-progress"

	// UpdatesDisabled is the state of a node which indicates the cluster
	// operator has indicated updates should not be managed by the update
	// operator.
	UpdatesDisabled = "disabled"

	// Key set by the update-agent to the current operator status of update_agent.
	//
	// Possible values are:
	//  - "UPDATE_STATUS_IDLE"
	//  - "UPDATE_STATUS_CHECKING_FOR_UPDATE"
	//  - "UPDATE_STATUS_UPDATE_AVAILABLE"
	//  - "UPDATE_STATUS_DOWNLOADING"
	//  - "UPDATE_STATUS_VERIFYING"
	//  - "UPDATE_STATUS_FINALIZING"
	//  - "UPDATE_STATUS_UPDATED_NEED_REBOOT"
	//  - "UPDATE_STATUS_REPORTING_ERROR_EVENT"
	//
	// It is possible, but extremely unlike for it to be "unknown status".
	AnnotationStatus string = Prefix + "status"

	// Key set by the update-agent to LAST_CHECKED_TIME reported by update_engine.
	//
	// It is zero if an update has never been checked for, or a UNIX timestamp.
	AnnotationLastCheckedTime = Prefix + "last-checked-time"

	// Key set by the update-agent to NEW_VERSION reported by update_engine.
	//
	// It is an opaque string, but might be semver.
	AnnotationNewVersion = Prefix + "new-version"

	// Key set by the update-agent to the value of "ID" in /etc/os-release.
	LabelID = Prefix + "id"

	// Key set by the update-agent to the value of "GROUP" in
	// /usr/share/coreos/update.conf, overridden by the value of "GROUP" in
	// /etc/coreos/update.conf.
	LabelGroup = Prefix + "group"

	// Key set by the update-agent to the value of "VERSION" in /etc/os-release.
	LabelVersion = Prefix + "version"
)
