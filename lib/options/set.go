package options

import "time"

// SetOptions is the standard options for Create/Update actions on the Calico
// API.
type SetOptions struct {
	// TTL for the datastore entry.
	// +optional
	TTL time.Duration
}

