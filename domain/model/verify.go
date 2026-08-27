package model

// VerifyStatus is the status of a verify job from
// GET /admin/datastore/{store}/verify/{upid}.
type VerifyStatus struct {
	UPID     string `json:"upid"`
	Store    string `json:"store,omitempty"`
	Status   string `json:"status,omitempty"`
	Errors   int    `json:"errors,omitempty"`
	Duration int    `json:"duration,omitempty"`
}
