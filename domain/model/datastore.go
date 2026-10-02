package model

// Datastore is a PBS datastore as returned by GET /admin/datastore.
type Datastore struct {
	Store       string `json:"store"`                  // datastore name (the "store" field)
	BackendType string `json:"backend-type,omitempty"` // filesystem|s3
	MountStatus string `json:"mount-status,omitempty"` // mounted|notmounted|nonremovable
	Comment     string `json:"comment,omitempty"`
	Maintenance string `json:"maintenance,omitempty"`
}

// DatastoreConfig is the full configuration of a single datastore as returned
// by GET /config/datastore/{name}.
type DatastoreConfig struct {
	Path                 string `json:"path"` // datastore backing directory (the "path" field)
	Comment              string `json:"comment,omitempty"`
	Backend              string `json:"backend,omitempty"`
	BackingDevice        string `json:"backing-device,omitempty"`
	KeepHourly           int    `json:"keep-hourly,omitempty"`
	KeepDaily            int    `json:"keep-daily,omitempty"`
	KeepWeekly           int    `json:"keep-weekly,omitempty"`
	KeepMonthly          int    `json:"keep-monthly,omitempty"`
	KeepYearly           int    `json:"keep-yearly,omitempty"`
	KeepLast             int    `json:"keep-last,omitempty"`
	GCSchedule           string `json:"gc-schedule,omitempty"`
	VerifyNew            bool   `json:"verify-new,omitempty"`
	PruneSchedule        string `json:"prune-schedule,omitempty"`
	NotificationMode     string `json:"notification-mode,omitempty"`
	NotifyUser           string `json:"notify-user,omitempty"`
	Notify               string `json:"notify,omitempty"`
	MaintenanceMode      string `json:"maintenance-mode,omitempty"`
	CounterResetSchedule string `json:"counter-reset-schedule,omitempty"`
	GCOnUnmount          bool   `json:"gc-on-unmount,omitempty"`
	Tuning               string `json:"tuning,omitempty"`
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
