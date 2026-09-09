package migrations

import (
	"io/fs"
	"sort"
	"testing"
)

func TestGroupModelAllowlistCompatibilityRunsBeforeRename(t *testing.T) {
	files, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	positions := map[string]int{}
	for i, name := range files {
		positions[name] = i
	}
	compat, ok := positions["234z_group_model_allowlist_compat.sql"]
	if !ok || compat >= positions["235_group_model_allowlist.sql"] {
		t.Fatal("compatibility migration must precede rename")
	}
}
