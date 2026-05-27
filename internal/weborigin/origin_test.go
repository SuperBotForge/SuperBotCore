package weborigin

import "testing"

func TestCanonicalizeList_NormalizesAndDeduplicatesOrigins(t *testing.T) {
	t.Parallel()

	got, err := CanonicalizeList([]string{
		" HTTPS://Schedule.Example.COM ",
		"https://schedule.example.com/",
		"http://localhost:5173",
		"",
	})
	if err != nil {
		t.Fatalf("CanonicalizeList() error = %v", err)
	}

	want := []string{"https://schedule.example.com", "http://localhost:5173"}
	if len(got) != len(want) {
		t.Fatalf("CanonicalizeList() len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CanonicalizeList()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCanonicalizeRejectsPath(t *testing.T) {
	t.Parallel()

	if _, err := Canonicalize("https://schedule.example.com/admin"); err == nil {
		t.Fatal("Canonicalize() error = nil, want path validation error")
	}
}

func TestFromURLExtractsOrigin(t *testing.T) {
	t.Parallel()

	got, err := FromURL("https://Schedule.Example.COM/admin?tab=main#top")
	if err != nil {
		t.Fatalf("FromURL() error = %v", err)
	}
	if got != "https://schedule.example.com" {
		t.Fatalf("FromURL() = %q, want https://schedule.example.com", got)
	}
}
