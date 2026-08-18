package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nixomose/zfstool/internal/api"
)

// LsblkCmd is the lsblk binary (overridable in tests).
var LsblkCmd = "lsblk"

// DfCmd is the df binary (overridable in tests).
var DfCmd = "df"

type lsblkDiskJSON struct {
	Blockdevices []lsblkNodeJSON `json:"blockdevices"`
}

type lsblkNodeJSON struct {
	Name       lsblkString     `json:"name"`
	KName      lsblkString     `json:"kname"`
	Size       lsblkUint       `json:"size"`
	Type       lsblkString     `json:"type"`
	Fstype     lsblkString     `json:"fstype"`
	Mountpoint lsblkString     `json:"mountpoint"`
	PkName     lsblkString     `json:"pkname"`
	PartLabel  lsblkString     `json:"partlabel"`
	Label      lsblkString     `json:"label"`
	UUID       lsblkString     `json:"uuid"`
	Rota       lsblkBool       `json:"rota"`
	Model      lsblkString     `json:"model"`
	Tran       lsblkString     `json:"tran"`
	Serial     lsblkString     `json:"serial"`
	Vendor     lsblkString     `json:"vendor"`
	Children   []lsblkNodeJSON `json:"children"`
}

// lsblkString accepts a JSON string, null, or a list of strings (newer lsblk).
type lsblkString string

func (s *lsblkString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*s = ""
		return nil
	}
	var v string
	if err := json.Unmarshal(b, &v); err == nil {
		*s = lsblkString(v)
		return nil
	}
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		if len(arr) > 0 {
			*s = lsblkString(arr[0])
		} else {
			*s = ""
		}
		return nil
	}
	*s = ""
	return nil
}

type lsblkUint uint64

func (u *lsblkUint) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || string(b) == `""` {
		*u = 0
		return nil
	}
	var n uint64
	if err := json.Unmarshal(b, &n); err == nil {
		*u = lsblkUint(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" || s == "-" {
			*u = 0
			return nil
		}
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			*u = 0
			return nil
		}
		*u = lsblkUint(v)
		return nil
	}
	*u = 0
	return nil
}

type lsblkBool struct {
	set bool
	val bool
}

func (b *lsblkBool) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		b.set = false
		return nil
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err == nil {
		b.set = true
		b.val = v
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		b.set = true
		b.val = n.String() != "0"
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			b.set = false
			return nil
		}
		b.set = true
		b.val = s == "1" || s == "true" || s == "yes"
		return nil
	}
	b.set = false
	return nil
}

func (b lsblkBool) ptr() *bool {
	if !b.set {
		return nil
	}
	v := b.val
	return &v
}

type dfRow struct {
	source string
	target string
	fstype string
	size   uint64
	used   uint64
	avail  uint64
}

// ListDisks returns physical disks (and leftover ZFS vdev leaves) with partition
// children, media type, and pool membership.
func ListDisks(ctx context.Context) ([]api.DiskSummary, error) {
	zfsByPath, _ := zfsDiskMembership(ctx)
	usage := collectDF(ctx)

	disks, err := listDisksLsblk(ctx)
	if err != nil || len(disks) == 0 {
		disks = listDisksSysfs()
	}
	if len(disks) == 0 {
		return listDisksZFSOnly(zfsByPath), nil
	}

	applyUsageAndMedia(disks, usage, "")
	attachZFSMembership(disks, zfsByPath)
	disks = appendUnmatchedZFS(disks, zfsByPath)
	sortDiskTree(disks)
	return disks, nil
}

// ListMounts returns mounted filesystems including non-ZFS volumes such as / and /boot.
func ListMounts(ctx context.Context) ([]api.MountEntry, error) {
	usage := collectDF(ctx)
	labelBySrc := map[string]api.DiskSummary{}
	if disks, err := listDisksLsblk(ctx); err == nil {
		applyUsageAndMedia(disks, usage, "")
		walkDisks(disks, func(d *api.DiskSummary) {
			labelBySrc[d.Device] = *d
			if d.Name != "" {
				labelBySrc["/dev/"+d.Name] = *d
			}
		})
	}

	seen := map[string]struct{}{}
	out := make([]api.MountEntry, 0, len(usage))
	for _, row := range usage {
		if skipMountFstype(row.fstype) {
			continue
		}
		if row.target == "" {
			continue
		}
		key := row.target + "\t" + row.source
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		m := api.MountEntry{
			Target: row.target,
			Source: row.source,
			Fstype: row.fstype,
			Size:   row.size,
			Used:   row.used,
			Avail:  row.avail,
		}
		if d, ok := labelBySrc[row.source]; ok {
			m.Label = firstNonEmpty(d.Label, d.PartLabel)
			m.UUID = d.UUID
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return mountOrder(out[i].Target) < mountOrder(out[j].Target)
	})
	return out, nil
}

func mountOrder(target string) string {
	if target == "/" {
		return "\x00"
	}
	return target
}

func skipMountFstype(fs string) bool {
	switch strings.ToLower(fs) {
	case "proc", "sysfs", "devtmpfs", "devpts", "tmpfs", "cgroup", "cgroup2",
		"pstore", "bpf", "debugfs", "tracefs", "securityfs", "hugetlbfs",
		"mqueue", "fusectl", "configfs", "overlay", "autofs", "rpc_pipefs",
		"binfmt_misc", "nsfs", "ramfs":
		return true
	default:
		return false
	}
}

func listDisksLsblk(ctx context.Context) ([]api.DiskSummary, error) {
	raw, err := runLsblk(ctx)
	if err != nil {
		return nil, err
	}
	return disksFromLsblkJSON(raw)
}

func runLsblk(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}
	name := LsblkCmd
	if _, err := exec.LookPath(name); err != nil && name == "lsblk" {
		if _, e := os.Stat("/usr/bin/lsblk"); e == nil {
			name = "/usr/bin/lsblk"
		}
	}
	cmd := exec.CommandContext(ctx, name, "-J", "-b", "-p",
		"-o", "NAME,KNAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,PKNAME,PARTLABEL,LABEL,UUID,ROTA,MODEL,TRAN,SERIAL,VENDOR")
	return cmd.Output()
}

func disksFromLsblkJSON(raw []byte) ([]api.DiskSummary, error) {
	var root lsblkDiskJSON
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	out := make([]api.DiskSummary, 0, len(root.Blockdevices))
	for i := range root.Blockdevices {
		n := convertLsblkNode(&root.Blockdevices[i], "")
		if n.Type == "loop" || n.Type == "rom" {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func convertLsblkNode(n *lsblkNodeJSON, parent string) api.DiskSummary {
	dev := strings.TrimSpace(string(n.Name))
	kname := strings.TrimSpace(string(n.KName))
	if kname == "" {
		kname = filepath.Base(dev)
	}
	if dev == "" && kname != "" {
		dev = "/dev/" + kname
	}
	d := api.DiskSummary{
		Device:     dev,
		Name:       kname,
		Type:       strings.ToLower(strings.TrimSpace(string(n.Type))),
		Size:       uint64(n.Size),
		Fstype:     string(n.Fstype),
		Mountpoint: string(n.Mountpoint),
		Label:      string(n.Label),
		PartLabel:  string(n.PartLabel),
		UUID:       string(n.UUID),
		Model:      strings.TrimSpace(string(n.Model)),
		Serial:     strings.TrimSpace(string(n.Serial)),
		Transport:  strings.ToLower(strings.TrimSpace(string(n.Tran))),
		Vendor:     strings.TrimSpace(string(n.Vendor)),
		Parent:     parent,
		Rotational: n.Rota.ptr(),
	}
	if d.Type == "" {
		d.Type = "disk"
	}
	if pk := strings.TrimSpace(string(n.PkName)); pk != "" && d.Parent == "" {
		if strings.HasPrefix(pk, "/dev/") {
			d.Parent = pk
		} else {
			d.Parent = "/dev/" + pk
		}
	}
	d.Media = mediaType(d.Rotational, d.Transport, d.Name)
	if len(n.Children) > 0 {
		d.Children = make([]api.DiskSummary, 0, len(n.Children))
		for i := range n.Children {
			d.Children = append(d.Children, convertLsblkNode(&n.Children[i], d.Device))
		}
	}
	return d
}

func mediaType(rotational *bool, transport, name string) string {
	t := strings.ToLower(transport)
	n := strings.ToLower(name)
	if strings.Contains(t, "nvme") || strings.HasPrefix(n, "nvme") {
		return "ssd"
	}
	if rotational != nil {
		if *rotational {
			return "hdd"
		}
		return "ssd"
	}
	if v, ok := sysfsRotational(name); ok {
		if v {
			return "hdd"
		}
		return "ssd"
	}
	return "unknown"
}

func sysfsRotational(kname string) (bool, bool) {
	kname = filepath.Base(strings.TrimSpace(kname))
	if kname == "" {
		return false, false
	}
	candidates := []string{
		"/sys/block/" + kname + "/queue/rotational",
		"/sys/class/block/" + kname + "/queue/rotational",
	}
	parent := wholeDiskKname(kname)
	if parent != kname {
		candidates = append(candidates,
			"/sys/block/"+parent+"/queue/rotational",
			"/sys/class/block/"+parent+"/queue/rotational",
		)
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		v := strings.TrimSpace(string(b))
		return v == "1", true
	}
	return false, false
}

func wholeDiskKname(kname string) string {
	base := filepath.Base(kname)
	if m := nvmePartRE.FindStringSubmatch(base); m != nil {
		return m[1]
	}
	if m := mmcPartRE.FindStringSubmatch(base); m != nil {
		return m[1]
	}
	if strings.HasPrefix(base, "md") || strings.HasPrefix(base, "dm-") || strings.HasPrefix(base, "loop") {
		return base
	}
	i := len(base)
	for i > 0 && base[i-1] >= '0' && base[i-1] <= '9' {
		i--
	}
	if i > 0 && i < len(base) && (strings.HasPrefix(base, "sd") || strings.HasPrefix(base, "vd") || strings.HasPrefix(base, "hd") || strings.HasPrefix(base, "xvd")) {
		return base[:i]
	}
	return base
}

func listDisksSysfs() []api.DiskSummary {
	ents, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil
	}
	var out []api.DiskSummary
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "sr") {
			continue
		}
		d := diskFromSysfs(name, "")
		if d.Device == "" {
			continue
		}
		out = append(out, d)
	}
	return out
}

func diskFromSysfs(kname, parent string) api.DiskSummary {
	dev := "/dev/" + kname
	d := api.DiskSummary{
		Device: dev,
		Name:   kname,
		Type:   "disk",
		Parent: parent,
	}
	if parent != "" {
		d.Type = "part"
	}
	if b, err := os.ReadFile("/sys/block/" + kname + "/size"); err == nil {
		if sectors, e := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64); e == nil {
			d.Size = sectors * 512
		}
	} else if parent != "" {
		pk := filepath.Base(parent)
		if b, err := os.ReadFile("/sys/block/" + pk + "/" + kname + "/size"); err == nil {
			if sectors, e := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64); e == nil {
				d.Size = sectors * 512
			}
		}
	}
	if rot, ok := sysfsRotational(kname); ok {
		r := rot
		d.Rotational = &r
	}
	if b, err := os.ReadFile("/sys/block/" + kname + "/device/model"); err == nil {
		d.Model = strings.TrimSpace(string(b))
	}
	d.Media = mediaType(d.Rotational, d.Transport, d.Name)
	// partitions
	base := "/sys/block/" + kname
	if parent != "" {
		return d
	}
	ents, err := os.ReadDir(base)
	if err != nil {
		return d
	}
	for _, e := range ents {
		n := e.Name()
		if !strings.HasPrefix(n, kname) || n == kname {
			continue
		}
		if _, err := os.Stat(base + "/" + n + "/partition"); err != nil {
			continue
		}
		d.Children = append(d.Children, diskFromSysfs(n, d.Device))
	}
	return d
}

func collectDF(ctx context.Context) []dfRow {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
	}
	name := DfCmd
	if _, err := exec.LookPath(name); err != nil && name == "df" {
		if _, e := os.Stat("/bin/df"); e == nil {
			name = "/bin/df"
		}
	}
	cmd := exec.CommandContext(ctx, name, "-P", "-B1", "-T")
	out, err := cmd.Output()
	if err != nil {
		return mountsFromProc()
	}
	rows := parseDF(out)
	if len(rows) == 0 {
		return mountsFromProc()
	}
	return rows
}

func parseDF(out []byte) []dfRow {
	var rows []dfRow
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	header := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if header {
			header = false
			continue
		}
		fields := strings.Fields(line)
		// Filesystem Type 1B-blocks Used Available Capacity Mounted on
		if len(fields) < 7 {
			continue
		}
		target := strings.Join(fields[6:], " ")
		row := dfRow{
			source: fields[0],
			fstype: fields[1],
			target: target,
		}
		row.size, _ = strconv.ParseUint(fields[2], 10, 64)
		row.used, _ = strconv.ParseUint(fields[3], 10, 64)
		row.avail, _ = strconv.ParseUint(fields[4], 10, 64)
		rows = append(rows, row)
	}
	return rows
}

func mountsFromProc() []dfRow {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	var rows []dfRow
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		rows = append(rows, dfRow{
			source: unescapeMount(fields[0]),
			target: unescapeMount(fields[1]),
			fstype: fields[2],
		})
	}
	return rows
}

func unescapeMount(s string) string {
	s = strings.ReplaceAll(s, `\040`, " ")
	s = strings.ReplaceAll(s, `\011`, "\t")
	s = strings.ReplaceAll(s, `\012`, "\n")
	s = strings.ReplaceAll(s, `\134`, `\`)
	return s
}

func applyUsageAndMedia(disks []api.DiskSummary, usage []dfRow, parentMedia string) {
	byTarget := map[string]dfRow{}
	bySource := map[string]dfRow{}
	for _, u := range usage {
		byTarget[u.target] = u
		bySource[u.source] = u
	}
	var walk func(d *api.DiskSummary, inheritedMedia string, inheritedRot *bool)
	walk = func(d *api.DiskSummary, inheritedMedia string, inheritedRot *bool) {
		if d.Rotational == nil && inheritedRot != nil {
			v := *inheritedRot
			d.Rotational = &v
		}
		if d.Media == "" || d.Media == "unknown" {
			if inheritedMedia != "" && inheritedMedia != "unknown" {
				d.Media = inheritedMedia
			} else {
				d.Media = mediaType(d.Rotational, d.Transport, d.Name)
			}
		}
		if u, ok := bySource[d.Device]; ok {
			d.Used = u.used
			d.Avail = u.avail
			if d.Mountpoint == "" {
				d.Mountpoint = u.target
			}
			if d.Fstype == "" {
				d.Fstype = u.fstype
			}
		} else if d.Mountpoint != "" {
			if u, ok := byTarget[d.Mountpoint]; ok {
				d.Used = u.used
				d.Avail = u.avail
			}
		}
		for i := range d.Children {
			rot := d.Rotational
			walk(&d.Children[i], d.Media, rot)
		}
	}
	for i := range disks {
		walk(&disks[i], parentMedia, disks[i].Rotational)
	}
}

func zfsDiskMembership(ctx context.Context) (map[string][]api.DiskMembership, error) {
	pools, err := ListPools(ctx)
	if err != nil {
		return map[string][]api.DiskMembership{}, err
	}
	byDev := map[string][]api.DiskMembership{}
	add := func(path string, m api.DiskMembership) {
		path = canonicalDevice(path)
		if path == "" {
			return
		}
		for _, existing := range byDev[path] {
			if existing.Pool == m.Pool && existing.Path == m.Path {
				return
			}
		}
		byDev[path] = append(byDev[path], m)
		base := filepath.Base(path)
		if base != "" {
			short := "/dev/" + base
			if short != path {
				byDev[short] = append(byDev[short], m)
			}
		}
	}
	for _, p := range pools {
		st, err := PoolStatusFull(ctx, p.Name)
		if err != nil {
			continue
		}
		enriched := enrichStatusLines(st.Config, p.Name)
		for _, ln := range enriched {
			if !ln.isDisk {
				continue
			}
			add(normalizeDiskPath(ln.name), api.DiskMembership{
				Pool:  p.Name,
				State: ln.state,
				Path:  ln.vdevPath,
			})
		}
	}
	return byDev, nil
}

func canonicalDevice(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != "" {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

func attachZFSMembership(disks []api.DiskSummary, byPath map[string][]api.DiskMembership) {
	walkDisks(disks, func(d *api.DiskSummary) {
		keys := []string{d.Device, canonicalDevice(d.Device)}
		if d.Name != "" {
			keys = append(keys, "/dev/"+d.Name)
		}
		seen := map[string]struct{}{}
		var mem []api.DiskMembership
		for _, k := range keys {
			for _, m := range byPath[k] {
				id := m.Pool + "\t" + m.Path
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				mem = append(mem, m)
			}
		}
		if len(mem) > 0 {
			d.Pools = mem
		}
	})
}

func appendUnmatchedZFS(disks []api.DiskSummary, byPath map[string][]api.DiskMembership) []api.DiskSummary {
	known := map[string]struct{}{}
	walkDisks(disks, func(d *api.DiskSummary) {
		known[d.Device] = struct{}{}
		known[canonicalDevice(d.Device)] = struct{}{}
		if d.Name != "" {
			known["/dev/"+d.Name] = struct{}{}
		}
	})
	var extra []api.DiskSummary
	seen := map[string]struct{}{}
	for path, mem := range byPath {
		c := canonicalDevice(path)
		if _, ok := known[path]; ok {
			continue
		}
		if _, ok := known[c]; ok {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		extra = append(extra, api.DiskSummary{
			Device: c,
			Name:   filepath.Base(c),
			Type:   "disk",
			Pools:  mem,
			Media:  mediaType(nil, "", filepath.Base(c)),
		})
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].Device < extra[j].Device })
	return append(disks, extra...)
}

func listDisksZFSOnly(byPath map[string][]api.DiskMembership) []api.DiskSummary {
	type acc struct {
		path string
		mem  []api.DiskMembership
	}
	by := map[string]*acc{}
	for path, mem := range byPath {
		c := canonicalDevice(path)
		a, ok := by[c]
		if !ok {
			a = &acc{path: c}
			by[c] = a
		}
		a.mem = append(a.mem, mem...)
	}
	out := make([]api.DiskSummary, 0, len(by))
	for _, a := range by {
		out = append(out, api.DiskSummary{
			Device: a.path,
			Name:   filepath.Base(a.path),
			Type:   "disk",
			Pools:  a.mem,
			Media:  mediaType(nil, "", filepath.Base(a.path)),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Device < out[j].Device })
	return out
}

func walkDisks(disks []api.DiskSummary, fn func(*api.DiskSummary)) {
	var walk func(d *api.DiskSummary)
	walk = func(d *api.DiskSummary) {
		fn(d)
		for i := range d.Children {
			walk(&d.Children[i])
		}
	}
	for i := range disks {
		walk(&disks[i])
	}
}

func sortDiskTree(disks []api.DiskSummary) {
	sort.Slice(disks, func(i, j int) bool { return disks[i].Device < disks[j].Device })
	var walk func(d *api.DiskSummary)
	walk = func(d *api.DiskSummary) {
		sort.Slice(d.Children, func(i, j int) bool { return d.Children[i].Device < d.Children[j].Device })
		for i := range d.Children {
			walk(&d.Children[i])
		}
	}
	for i := range disks {
		walk(&disks[i])
	}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
