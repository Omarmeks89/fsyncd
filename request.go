// request contain request schemas
package main

// SyncDirectoriesRequest query for start directories sync
type SyncDirectoriesRequest struct {
	SrcPath        string `json:"src_path" Validate:"required,dirpath"`
	DstPath        string `json:"dst_path" Validate:"required,dirpath"`
	MaxDiffPercent int    `json:"max_diff_percent" Validate:"required,gt=0,lte=100"`
}
