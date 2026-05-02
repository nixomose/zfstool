package collector

import "testing"

func TestParseStatusBasic(t *testing.T) {
	raw := `  pool: tank
 state: ONLINE
  scan: scrub repaired 0B in 0 days 00:00:00 on Sun Jan  1 00:00:00 2020
config:

	NAME        STATE     READ WRITE CKSUM
	tank        ONLINE       0     0     0
	  mirror-0  ONLINE       0     0     0
	    sda     ONLINE       0     0     0
	    sdb     ONLINE       0     0     0

errors: No known data errors
`
	st := parseStatus(raw)
	if st.State != "ONLINE" {
		t.Fatalf("state %q", st.State)
	}
	if st.Scan == nil {
		t.Fatal("expected scan")
	}
	if len(st.Config) < 2 {
		t.Fatalf("config lines %d", len(st.Config))
	}
}
