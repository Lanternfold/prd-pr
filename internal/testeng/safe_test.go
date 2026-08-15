package testeng

import (
	"testing"

	"github.com/lanternfold/prd-pr/internal/fsguard"
)

func TestCommandSafeRejectsChdir(t *testing.T) {
	root := t.TempDir()
	jail, err := fsguard.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := commandSafe(jail, Command{Program: "go", Args: []string{"test", "-C", "/etc"}}); err == nil {
		t.Fatal("expected escape failure")
	}
	if err := commandSafe(jail, Command{Program: "go", Args: []string{"test", "./..."}}); err != nil {
		t.Fatal(err)
	}
}
