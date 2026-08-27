package model

// Datastore is a PBS datastore as returned by GET /admin/datastore.
type Datastore struct {
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	Comment     string `json:"comment,omitempty"`
	KeepDaily   int    `json:"keep-daily,omitempty"`
	KeepWeekly  int    `json:"keep-weekly,omitempty"`
	KeepMonthly int    `json:"keep-monthly,omitempty"`
	KeepYearly  int    `json:"keep-yearly,omitempty"`
	NotifyUser  string `json:"notify-user,omitempty"`
	GC          string `json:"gc-schedule,omitempty"`
	Verify      string `json:"verify-new,omitempty"`
}

// DatastoreStatus is the status of a single datastore from
// GET /admin/datastore/{store}/status.
type DatastoreStatus struct {
	Store       string  `json:"store"`
	Total       int64   `json:"total,omitempty"`
	Used        int64   `json:"used,omitempty"`
	Avail       int64   `json:"avail,omitempty"`
	UsedFrac    float64 `json:"used_fraction,omitempty"`
	Snapshots   int     `json:"snapshots,omitempty"`
	Chunks      int     `json:"chunks,omitempty"`
	Maintenance string  `json:"maintenance,omitempty"`
}
