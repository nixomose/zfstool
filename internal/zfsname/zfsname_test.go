package zfsname

import (
	"errors"
	"testing"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"tank", false},
		{"tank/ds", false},
		{"tank@snap", false},
		{"", true},
		{"-c,iostat-10s", true},
		{"-x", true},
		{"pool\nname", true},
	}
	for _, c := range cases {
		err := Check("pool", c.name)
		if c.wantErr && err == nil {
			t.Fatalf("%q: expected error", c.name)
		}
		if !c.wantErr && err != nil {
			t.Fatalf("%q: unexpected %v", c.name, err)
		}
		if c.wantErr && err != nil && !errors.Is(err, ErrInvalid) {
			t.Fatalf("%q: want ErrInvalid, got %v", c.name, err)
		}
	}
}

func TestAppend(t *testing.T) {
	got, err := Append([]string{"status", "-P"}, "pool", "tank")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"status", "-P", "--", "tank"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
	if _, err := Append(nil, "pool", "-evil"); err == nil {
		t.Fatal("expected error")
	}
}
