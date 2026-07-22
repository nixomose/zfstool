package api

import "time"

// HostInfo is agent + OS snapshot for GetHost.
type HostInfo struct {
	Hostname       string            `json:"hostname"`
	OSName         string            `json:"osName,omitempty"`
	OSVersion      string            `json:"osVersion,omitempty"`
	Kernel         string            `json:"kernel,omitempty"`
	Arch           string            `json:"arch,omitempty"`
	UptimeSeconds  float64           `json:"uptimeSeconds,omitempty"`
	MemTotalKB     uint64            `json:"memTotalKb,omitempty"`
	MemAvailableKB uint64            `json:"memAvailableKb,omitempty"`
	ZFSVersions    []string          `json:"zfsVersions,omitempty"`
	ZFSMismatch    bool              `json:"zfsMismatch,omitempty"`
	ARC            map[string]uint64 `json:"arc,omitempty"`
	AgentVersion   string            `json:"agentVersion"`
	CollectedAt    time.Time         `json:"collectedAt"`
}

// PoolSummary is a row from zpool list.
type PoolSummary struct {
	Name         string  `json:"name"`
	Size         uint64  `json:"size"`
	Allocated    uint64  `json:"allocated"`
	Free         uint64  `json:"free"`
	Fragmentation *uint64 `json:"fragmentationPct,omitempty"`
	CapacityPct  *uint64 `json:"capacityPct,omitempty"`
	DedupRatio   string  `json:"dedupRatio,omitempty"`
	Health       string  `json:"health"`
	AltRoot      string  `json:"altroot,omitempty"`
}

// PoolStatus is parsed zpool status -P style tree + raw lines for maintenance.
type PoolStatus struct {
	Pool       string       `json:"pool"`
	State      string       `json:"state"`
	Config     []StatusLine `json:"config"`
	Errors     string       `json:"errors,omitempty"`
	ScanRaw    string       `json:"scanRaw,omitempty"`
	Scan       *ScanInfo    `json:"scan,omitempty"`
	Checkpoint *Checkpoint  `json:"checkpoint,omitempty"`
	Raw        string       `json:"raw,omitempty"`
}

// StatusLine is one line in zpool status config output.
type StatusLine struct {
	Indent int    `json:"indent"`
	Name   string `json:"name"`
	State  string `json:"state,omitempty"`
	Read   string `json:"read,omitempty"`
	Write  string `json:"write,omitempty"`
	Cksum  string `json:"cksum,omitempty"`
}

// ScanInfo summarizes scrub/resilver from status.
type ScanInfo struct {
	Type           string `json:"type"` // scrub, resilver, trim, unknown
	InProgress     bool   `json:"inProgress"`
	Progress       string `json:"progress,omitempty"`
	BytesProcessed string `json:"bytesProcessed,omitempty"`
	ETA            string `json:"eta,omitempty"`
	LastRun        string `json:"lastRun,omitempty"`
	Errors         string `json:"errors,omitempty"`
	Raw            string `json:"raw,omitempty"`
}

// Checkpoint from zpool status / get.
type Checkpoint struct {
	Active bool   `json:"active"`
	Name   string `json:"name,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// DatasetRow from zfs list.
type DatasetRow struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Used          uint64            `json:"used"`
	Avail         uint64            `json:"avail"`
	Refer         uint64            `json:"refer"`
	Mountpoint    string            `json:"mountpoint,omitempty"`
	Origin        string            `json:"origin,omitempty"`
	Properties    map[string]string `json:"properties,omitempty"`
	PropertySource map[string]string `json:"propertySource,omitempty"`
}

// PoolHistoryEntry is one zpool history line.
type PoolHistoryEntry struct {
	Time    string `json:"time"`
	Command string `json:"command"`
}

// MaintenanceBundle per pool (scrub/resilver/trim props + scan).
type MaintenanceBundle struct {
	Pool     string            `json:"pool"`
	Autotrim string            `json:"autotrim,omitempty"`
	Scan     *ScanInfo         `json:"scan,omitempty"`
	Props    map[string]string `json:"props,omitempty"`
}

// IOStatSample from zpool iostat one line (simplified).
type IOStatSample struct {
	Pool  string   `json:"pool"`
	VDev  string   `json:"vdev,omitempty"`
	CapOpsRead  float64 `json:"capOpsRead,omitempty"`
	CapOpsWrite float64 `json:"capOpsWrite,omitempty"`
	BandwidthRead  string `json:"bandwidthRead,omitempty"`
	BandwidthWrite string `json:"bandwidthWrite,omitempty"`
	Raw   []string `json:"raw,omitempty"`
}

// DiskMembership is one pool that uses a block device as a vdev leaf.
type DiskMembership struct {
	Pool  string `json:"pool"`
	State string `json:"state,omitempty"`
	Path  string `json:"path,omitempty"` // topology path under the pool
}

// DiskSummary is a unique block device found across pool status trees.
type DiskSummary struct {
	Device string           `json:"device"`
	Pools  []DiskMembership `json:"pools,omitempty"`
}

// SMARTDisk is placeholder structure; filled by smartctl later.
type SMARTDisk struct {
	Device string                 `json:"device"`
	JSON   map[string]interface{} `json:"json,omitempty"`
	Error  string                 `json:"error,omitempty"`
}

// ErrorBody is JSON error envelope.
type ErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// DatasetGraphNode for snapshot/clone DAG (lightweight v1).
type DatasetGraphNode struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Origin   string   `json:"origin,omitempty"`
	Children []string `json:"children,omitempty"`
}
