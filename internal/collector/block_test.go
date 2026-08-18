package collector

import (
	"testing"

	"github.com/nixomose/zfstool/internal/api"
)

func TestDisksFromLsblkJSON(t *testing.T) {
	raw := []byte(`{
  "blockdevices": [
    {
      "name": "/dev/sda",
      "kname": "sda",
      "size": 500107862016,
      "type": "disk",
      "fstype": null,
      "mountpoint": null,
      "rota": true,
      "model": "ST500DM002",
      "tran": "sata",
      "serial": "Z1",
      "children": [
        {
          "name": "/dev/sda1",
          "kname": "sda1",
          "size": 536870912,
          "type": "part",
          "fstype": "vfat",
          "mountpoint": "/boot/efi",
          "pkname": "sda",
          "partlabel": "EFI",
          "rota": 1
        },
        {
          "name": "/dev/sda2",
          "kname": "sda2",
          "size": 1073741824,
          "type": "part",
          "fstype": "ext4",
          "mountpoint": ["/boot"],
          "rota": "1"
        },
        {
          "name": "/dev/sda3",
          "kname": "sda3",
          "size": 498497499136,
          "type": "part",
          "fstype": "zfs_member",
          "mountpoint": null,
          "rota": true
        }
      ]
    },
    {
      "name": "/dev/nvme0n1",
      "kname": "nvme0n1",
      "size": 256060514304,
      "type": "disk",
      "rota": false,
      "model": "Samsung SSD",
      "tran": "nvme",
      "children": [
        {
          "name": "/dev/nvme0n1p1",
          "kname": "nvme0n1p1",
          "size": 256060514304,
          "type": "part",
          "fstype": "zfs_member",
          "rota": false
        }
      ]
    },
    {
      "name": "/dev/loop0",
      "kname": "loop0",
      "size": 1234,
      "type": "loop"
    }
  ]
}`)
	disks, err := disksFromLsblkJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(disks) != 2 {
		t.Fatalf("want 2 disks (loop skipped), got %d", len(disks))
	}
	sda := disks[0]
	if sda.Device != "/dev/sda" || sda.Media != "hdd" {
		t.Fatalf("sda: device=%s media=%s", sda.Device, sda.Media)
	}
	if sda.Rotational == nil || !*sda.Rotational {
		t.Fatal("sda should be rotational")
	}
	if len(sda.Children) != 3 {
		t.Fatalf("sda children %d", len(sda.Children))
	}
	if sda.Children[0].Mountpoint != "/boot/efi" || sda.Children[0].Fstype != "vfat" {
		t.Fatalf("efi part: %+v", sda.Children[0])
	}
	if sda.Children[1].Mountpoint != "/boot" {
		t.Fatalf("boot mount %q", sda.Children[1].Mountpoint)
	}
	nvme := disks[1]
	if nvme.Media != "ssd" {
		t.Fatalf("nvme media %s", nvme.Media)
	}
	if nvme.Rotational == nil || *nvme.Rotational {
		t.Fatal("nvme should not be rotational")
	}
}

func TestMediaType(t *testing.T) {
	tru, fal := true, false
	if g := mediaType(&tru, "sata", "sda"); g != "hdd" {
		t.Fatalf("sata hdd: %s", g)
	}
	if g := mediaType(&fal, "sata", "sda"); g != "ssd" {
		t.Fatalf("sata ssd: %s", g)
	}
	if g := mediaType(&tru, "nvme", "nvme0n1"); g != "ssd" {
		t.Fatalf("nvme overrides rota: %s", g)
	}
	if g := mediaType(nil, "", "nvme1n1"); g != "ssd" {
		t.Fatalf("nvme name: %s", g)
	}
}

func TestParseDF(t *testing.T) {
	raw := []byte(`Filesystem     Type     1B-blocks      Used Available Capacity Mounted on
/dev/sda2      ext4     1073741824  536870912 536870912      50% /boot
tank/root      zfs      2000000000  500000000 1500000000     25% /
tmpfs          tmpfs      16410200         0  16410200       0% /dev/shm
`)
	rows := parseDF(raw)
	if len(rows) != 3 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0].source != "/dev/sda2" || rows[0].target != "/boot" || rows[0].fstype != "ext4" {
		t.Fatalf("row0 %+v", rows[0])
	}
	if rows[0].size != 1073741824 || rows[0].used != 536870912 {
		t.Fatalf("sizes %+v", rows[0])
	}
	if rows[1].target != "/" || rows[1].fstype != "zfs" {
		t.Fatalf("root %+v", rows[1])
	}
}

func TestSkipMountFstype(t *testing.T) {
	if !skipMountFstype("tmpfs") || !skipMountFstype("cgroup2") {
		t.Fatal("should skip virtual fs")
	}
	if skipMountFstype("ext4") || skipMountFstype("zfs") || skipMountFstype("vfat") {
		t.Fatal("should keep real volumes")
	}
}

func TestParseDatasetListUsedBy(t *testing.T) {
	raw := []byte("tank\tfilesystem\t100\t50\t40\t/\t-\t40\t20\t40\t0\n" +
		"tank/data\tfilesystem\t40\t50\t40\t/data\t-\t30\t10\t0\t0\n" +
		"tank/data@snap\tsnapshot\t10\t0\t40\t-\t-\t0\t0\t0\t0\n")
	rows := parseDatasetList(raw)
	if len(rows) != 3 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0].UsedByDataset != 40 || rows[0].UsedBySnapshots != 20 || rows[0].UsedByChildren != 40 {
		t.Fatalf("tank used-by %+v", rows[0])
	}
	if rows[1].Mountpoint != "/data" || rows[1].Origin != "" {
		t.Fatalf("data mp/origin %+v", rows[1])
	}
	if rows[2].Type != "snapshot" || rows[2].Used != 10 {
		t.Fatalf("snap %+v", rows[2])
	}
}

func TestWholeDiskKname(t *testing.T) {
	cases := map[string]string{
		"sda1":      "sda",
		"nvme0n1p3": "nvme0n1",
		"nvme0n1":   "nvme0n1",
		"mmcblk0p1": "mmcblk0",
		"md0":       "md0",
		"vda2":      "vda",
	}
	for in, want := range cases {
		if g := wholeDiskKname(in); g != want {
			t.Errorf("%s: got %s want %s", in, g, want)
		}
	}
}

func TestAttachZFSMembershipOnPartition(t *testing.T) {
	raw := []byte(`{"blockdevices":[{"name":"/dev/sda","kname":"sda","size":100,"type":"disk","rota":true,"children":[{"name":"/dev/sda3","kname":"sda3","size":90,"type":"part","fstype":"zfs_member"}]}]}`)
	disks, err := disksFromLsblkJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string][]api.DiskMembership{
		"/dev/sda3": {{Pool: "tank", State: "ONLINE", Path: "mirror-0"}},
	}
	attachZFSMembership(disks, by)
	if len(disks[0].Children) != 1 || len(disks[0].Children[0].Pools) != 1 {
		t.Fatalf("membership not attached: %+v", disks[0].Children)
	}
	if disks[0].Children[0].Pools[0].Pool != "tank" {
		t.Fatalf("pool %v", disks[0].Children[0].Pools)
	}
}

func TestApplyUsageAndMedia(t *testing.T) {
	raw := []byte(`{"blockdevices":[{"name":"/dev/sda","kname":"sda","size":1000,"type":"disk","rota":true,"children":[{"name":"/dev/sda1","kname":"sda1","size":200,"type":"part","fstype":"ext4","mountpoint":"/boot"}]}]}`)
	disks, err := disksFromLsblkJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	applyUsageAndMedia(disks, []dfRow{{
		source: "/dev/sda1",
		target: "/boot",
		fstype: "ext4",
		size:   200,
		used:   50,
		avail:  150,
	}}, "")
	p := disks[0].Children[0]
	if p.Used != 50 || p.Avail != 150 {
		t.Fatalf("usage %+v", p)
	}
	if p.Media != "hdd" {
		t.Fatalf("inherited media %s", p.Media)
	}
}
