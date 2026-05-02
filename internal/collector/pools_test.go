package collector

import "testing"

func TestParseMaybePercent(t *testing.T) {
	p, err := parseMaybePercent("42%")
	if err != nil || p == nil || *p != 42 {
		t.Fatalf("got %v %v", p, err)
	}
	p, err = parseMaybePercent("-")
	if err != nil || p != nil {
		t.Fatalf("want nil, got %v", p)
	}
}
