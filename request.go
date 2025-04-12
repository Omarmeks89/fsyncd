// request contain request schemas
package main

// SyncDirectoriesRequest query for start directories sync
type SyncDirectoriesRequest struct {
	SrcPath        string `json:"src_path" validate:"required,dirpath"`
	DstPath        string `json:"dst_path" validate:"required,dirpath"`
	MaxDiffPercent int    `json:"max_diff_percent" validate:"required,gt=0,lte=100"`
}

// ChangeSyncConfig query for change sync configuration
type ChangeSyncConfig struct {
	SyncDirectoriesRequest
	SyncTime string `json:"sync_time" validate:"required"`
}
