package human

import "testing"

func TestOneCredentialAtATime(t *testing.T) {
	root := t.TempDir()
	names := []string{"gh", "openai"}
	if got := NextMissingCredential(names, root); got != "gh" {
		t.Fatalf("got %q", got)
	}
	if err := RecordCredential(root, "gh", CredPresentUnverified); err != nil {
		t.Fatal(err)
	}
	if got := NextMissingCredential(names, root); got != "openai" {
		t.Fatalf("got %q", got)
	}
}

func TestRequestRoundTrip(t *testing.T) {
	root := t.TempDir()
	req := Request{Reason: "repair exhausted", Kind: KindRepairFail, Needed: "inspect tests", Phase: "P1"}
	if err := WriteRequest(root, req); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRequest(root)
	if err != nil || got.Needed != req.Needed || got.ID == "" {
		t.Fatalf("%+v %v", got, err)
	}
}
