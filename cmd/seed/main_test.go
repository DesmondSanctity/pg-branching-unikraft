package main

import "testing"

func TestNoteCountBounds(t *testing.T) {
	for i := 0; i < 10000; i++ {
		if n := noteCount(); n < 10 || n > 200 {
			t.Fatalf("noteCount() = %d, want 10..200", n)
		}
	}
}

func TestRandCompanyInSet(t *testing.T) {
	set := map[string]bool{
		"Acme": true, "Globex": true, "Initech": true, "Umbrella": true, "Hooli": true,
		"Stark": true, "Wayne": true, "Wonka": true, "Cyberdyne": true, "Soylent": true,
	}
	for i := 0; i < 200; i++ {
		if c := randCompany(); !set[c] {
			t.Fatalf("randCompany() = %q, not in the known set", c)
		}
	}
}

func TestRandBodyNonEmpty(t *testing.T) {
	for i := 0; i < 200; i++ {
		if randBody() == "" {
			t.Fatal("randBody() returned empty")
		}
	}
}
