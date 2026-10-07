package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultIsReadOnlyPreview(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "test.html")
	original := []byte(`<script src="js/auth.js"></script>`)
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"-root", root, "-operation", "update-html-scripts"}, &output); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(original, unchanged) || !bytes.Contains(output.Bytes(), []byte("Preview only")) {
		t.Fatal("default command modified the project")
	}
	if err := run([]string{"-root", root, "-operation", "update-html-scripts", "-apply"}, &output); err != nil {
		t.Fatal(err)
	}
	changed, _ := os.ReadFile(path)
	if !bytes.Contains(changed, []byte(`type="module" src="js/pages/auth.js"`)) {
		t.Fatal("explicit apply did not migrate the tag")
	}
}

func TestInvalidOperationAndExtraArgumentsStop(t *testing.T) {
	var output bytes.Buffer
	root := t.TempDir()
	for _, args := range [][]string{{"-root", root, "-operation", "unknown"}, {"-root", root, "-operation", "update-html-css", "unexpected"}} {
		if err := run(args, &output); err == nil {
			t.Fatal("invalid command accepted")
		}
	}
}
