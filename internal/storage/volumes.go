package storage

// Volume describes a currently mounted external storage volume.
type Volume struct {
	Path      string `json:"path"`
	Label     string `json:"label"`
	UUID      string `json:"uuid"`
	FSType    string `json:"fsType"`
	TotalSize int64  `json:"totalSize"`
	UsedSize  int64  `json:"usedSize"`
	External  bool   `json:"external"`
}
