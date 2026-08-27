package model

// PruneStatus is the status of a prune job from
// GET /admin/datastore/{store}/prune/{upid}.
type PruneStatus struct {
	UPID     string `json:"upid"`
	Store    string `json:"store,omitempty"`
	Status   string `json:"status,omitempty"`
	Duration int    `json:"duration,omitempty"`
}
