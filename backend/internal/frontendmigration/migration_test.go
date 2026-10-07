package frontendmigration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func copyFixture(t *testing.T, source, target string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".frontend-migrate-backups" {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = text(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestOriginalPythonFixtureParity(t *testing.T) {
	for _, operation := range []string{"refactor-js", "refactor-pages", "update-html-css", "update-html-scripts"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			copyFixture(t, "testdata/legacy", root)
			if operation == "refactor-pages" {
				if err := os.Remove(filepath.Join(root, "js/pages/dashboard.js")); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshot(t, root)
			plan, err := Prepare(root, operation)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Changes) == 0 {
				t.Fatal("fixture must exercise a real transformation")
			}
			if after := snapshot(t, root); !equalFiles(before, after) {
				t.Fatal("planning modified source files")
			}
			backup, err := Apply(plan)
			if err != nil {
				t.Fatal(err)
			}
			if backup == "" {
				t.Fatal("changed files must be backed up")
			}
			for _, change := range plan.Changes {
				if change.Existed {
					original, err := os.ReadFile(filepath.Join(backup, filepath.FromSlash(change.Path)))
					if err != nil || !bytes.Equal(original, change.Before) {
						t.Fatalf("backup mismatch: %s (%v)", change.Path, err)
					}
				}
			}
			encoded, err := os.ReadFile("testdata/" + operation + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var expected map[string]string
			if err := json.Unmarshal(encoded, &expected); err != nil {
				t.Fatal(err)
			}
			if operation == "refactor-pages" {
				// Correct the original script's two broken admin relative imports.
				expected["js/pages/admin.js"] = strings.ReplaceAll(strings.ReplaceAll(expected["js/pages/admin.js"], "from './api/client.js'", "from '../api/client.js'"), "from './components/toast.js'", "from '../components/toast.js'")
			}
			actual := snapshot(t, root)
			if !equalFiles(actual, expected) {
				for name, want := range expected {
					if actual[name] != want {
						t.Errorf("output differs from original fixture: %s\nwant: %q\ngot: %q", name, want, actual[name])
					}
				}
				if len(actual) != len(expected) {
					t.Errorf("file inventory mismatch: %d vs %d", len(actual), len(expected))
				}
			}
			again, err := Prepare(root, operation)
			if operation == "refactor-js" {
				if err == nil {
					t.Fatal("legacy component generator must refuse an already migrated tree")
				}
			} else if err != nil || len(again.Changes) != 0 {
				t.Fatalf("rerun must not change migrated files: %v", err)
			}
		})
	}
}

func equalFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, ok := b[key]; !ok || value != other {
			return false
		}
	}
	return true
}

func TestRefusesExistingModernModulesBeforeWriting(t *testing.T) {
	root := t.TempDir()
	copyFixture(t, "testdata/legacy", root)
	path := filepath.Join(root, "js/components/sidebar.js")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("// modern wallet security fixes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	if _, err := Prepare(root, "refactor-js"); err == nil {
		t.Fatal("existing component would be overwritten")
	}
	if !equalFiles(before, snapshot(t, root)) {
		t.Fatal("failed preflight changed files")
	}
	if _, err := Prepare(root, "refactor-pages"); err == nil {
		t.Fatal("existing page would be overwritten")
	}
}

func TestChangedSourceInvalidatesPlan(t *testing.T) {
	root := t.TempDir()
	copyFixture(t, "testdata/legacy", root)
	plan, err := Prepare(root, "refactor-js")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "js/api.js"), []byte("// changed after preview\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	if _, err := Apply(plan); err == nil {
		t.Fatal("stale plan was applied")
	}
	if !equalFiles(before, snapshot(t, root)) {
		t.Fatal("stale plan changed files")
	}
}

func TestWriteFailureRollsBackAndPreservesBackups(t *testing.T) {
	root := t.TempDir()
	copyFixture(t, "testdata/legacy", root)
	before := snapshot(t, root)
	plan, err := Prepare(root, "refactor-js")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	backup, err := apply(plan, func(path string, data []byte, mode fs.FileMode) error {
		calls++
		if calls == 2 {
			return errors.New("simulated disk failure")
		}
		return atomicWrite(path, data, mode)
	})
	if err == nil || backup == "" {
		t.Fatal("write failure must report an error and backup location")
	}
	if !equalFiles(before, snapshot(t, root)) {
		t.Fatal("write failure did not restore original files")
	}
}

func TestRejectsSymlinksAndEscapingPaths(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "page.html"), []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Root: root}
	if _, err := plan.safePath("../outside"); err == nil {
		t.Fatal("escaping path accepted")
	}
	if _, err := plan.safePath(outside); err == nil {
		t.Fatal("absolute path accepted")
	}
	if err := os.Symlink(filepath.Join(outside, "page.html"), filepath.Join(root, "page.html")); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatalf("symlink protection must be exercised on this platform: %v", err)
		}
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := Prepare(root, "update-html-css"); err == nil {
		t.Fatal("symlink accepted")
	}
	data, _ := os.ReadFile(filepath.Join(outside, "page.html"))
	if string(data) != "outside" {
		t.Fatal("outside file changed")
	}
}

func TestActiveMigrationLockPreventsWrites(t *testing.T) {
	root := t.TempDir()
	copyFixture(t, "testdata/legacy", root)
	plan, err := Prepare(root, "refactor-js")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".frontend-migrate.lock"), []byte("active migration"), 0600); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	if _, err := Apply(plan); err == nil {
		t.Fatal("active migration lock ignored")
	}
	if !equalFiles(before, snapshot(t, root)) {
		t.Fatal("locked migration changed files or removed another migration's lock")
	}
}

func TestInvalidUTF8StopsCompletePlan(t *testing.T) {
	root := t.TempDir()
	copyFixture(t, "testdata/legacy", root)
	if err := os.WriteFile(filepath.Join(root, "z-invalid.html"), []byte{0xff}, 0644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	if _, err := Prepare(root, "update-html-css"); err == nil {
		t.Fatal("invalid input accepted after planning earlier changes")
	}
	if !equalFiles(before, snapshot(t, root)) {
		t.Fatal("invalid input changed earlier files")
	}
}

func TestPreservesCRLFAndRepairsMissingTopbarImport(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "js/pages/dashboard.js")
	if err := os.MkdirAll(filepath.Dir(page), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("import '../components/sidebar.js';\r\n// page\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	plan, err := Prepare(root, "update-html-scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	result, _ := os.ReadFile(page)
	if !strings.Contains(string(result), "import '../components/topbar.js';\r\n") {
		t.Fatal("missing topbar import was not repaired")
	}
	if bytes.Contains(bytes.ReplaceAll(result, []byte("\r\n"), nil), []byte("\n")) {
		t.Fatal("mixed newline conventions")
	}
}
