// Package frontendmigration replaces the four historical Python refactoring
// scripts. It plans changes without writing and only applies validated plans.
package frontendmigration

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

//go:embed templates/*.json
var templates embed.FS

type Change struct {
	Path    string      `json:"path"`
	Delete  bool        `json:"delete,omitempty"`
	Existed bool        `json:"existed"`
	Before  []byte      `json:"-"`
	After   []byte      `json:"-"`
	Mode    fs.FileMode `json:"mode"`
}

type Plan struct {
	Root    string
	Changes []Change
}

func Prepare(root, operation string) (*Plan, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("root must be a real directory")
	}
	plan := &Plan{Root: absolute}
	switch operation {
	case "refactor-js":
		err = plan.refactorJS()
	case "refactor-pages":
		err = plan.refactorPages()
	case "update-html-css":
		err = plan.updateCSS()
	case "update-html-scripts":
		err = plan.updateScripts()
	default:
		err = fmt.Errorf("unknown operation %q", operation)
	}
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// Reject paths that escape the selected project or traverse symlinks/junctions.
func (p *Plan) safePath(relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", errors.New("absolute target path is forbidden")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("target must stay inside the project")
	}
	current := p.Root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing symlink: %s", relative)
		}
	}
	return current, nil
}

func (p *Plan) read(relative string) ([]byte, fs.FileMode, error) {
	target, err := p.safePath(relative)
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("not a regular file: %s", relative)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, 0, err
	}
	if !utf8.Valid(data) {
		return nil, 0, fmt.Errorf("invalid UTF-8: %s", relative)
	}
	return data, info.Mode().Perm(), nil
}

func text(data []byte) string { return strings.ReplaceAll(string(data), "\r\n", "\n") }

func (p *Plan) write(relative, content string, createOnly bool) error {
	before, mode, err := p.read(relative)
	existed := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if createOnly && existed {
		return fmt.Errorf("refusing to overwrite existing module: %s", relative)
	}
	if mode == 0 {
		mode = 0644
	}
	// Preserve the newline convention of an existing file.
	if bytes.Contains(before, []byte("\r\n")) {
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}
	after := []byte(content)
	if existed && bytes.Equal(before, after) {
		return nil
	}
	p.Changes = append(p.Changes, Change{Path: relative, Existed: existed, Before: before, After: after, Mode: mode})
	return nil
}

func (p *Plan) remove(relative string) error {
	before, mode, err := p.read(relative)
	if err != nil {
		return err
	}
	p.Changes = append(p.Changes, Change{Path: relative, Delete: true, Existed: true, Before: before, Mode: mode})
	return nil
}

func (p *Plan) refactorJS() error {
	api, _, err := p.read("js/api.js")
	if err != nil {
		return fmt.Errorf("legacy API source required; current modules are not regenerated: %w", err)
	}
	if _, _, err := p.read("js/components.js"); err != nil {
		return err
	}
	if !strings.Contains(text(api), "window.ApiClient = ApiClient;") {
		return errors.New("legacy API export not recognized")
	}
	if err := p.write("js/api/client.js", strings.ReplaceAll(text(api), "window.ApiClient = ApiClient;", "export default ApiClient;"), true); err != nil {
		return err
	}
	for _, component := range []string{"toast", "sidebar", "topbar"} {
		data, err := templates.ReadFile("templates/" + component + ".json")
		if err != nil {
			return err
		}
		var source string
		if err := json.Unmarshal(data, &source); err != nil {
			return err
		}
		if err := p.write("js/components/"+component+".js", source, true); err != nil {
			return err
		}
	}
	if err := p.remove("js/api.js"); err != nil {
		return err
	}
	return p.remove("js/components.js")
}

func addImports(content string, imports ...string) string {
	prefix := ""
	for _, statement := range imports {
		if !strings.Contains(content, statement) {
			prefix += statement + "\n"
		}
	}
	if prefix != "" {
		return prefix + "\n" + content
	}
	return content
}

func (p *Plan) refactorPages() error {
	pages := []struct{ source, name, exports string }{
		{"js/auth.js", "auth", ""}, {"js/dashboard.js", "dashboard", ""},
		{"js/market.js", "market", "\nwindow.submitPrediction = submitPrediction;\nwindow.postComment = postComment;\n"},
		{"js/profile.js", "profile", ""},
		{"js/wallet.js", "wallet", "\nwindow.openWithdraw = openWithdraw;\n\nwindow.startKYC = startKYC;\n"},
		{"admin.js", "admin", "\nwindow.switchTab = switchTab;\nwindow.fetchProposedMarkets = fetchProposedMarkets;\nwindow.approveMarket = approveMarket;\nwindow.resolveMarket = resolveMarket;\nwindow.fetchKycRequests = fetchKycRequests;\nwindow.reviewKyc = reviewKyc;\nwindow.fetchWithdrawals = fetchWithdrawals;\nwindow.approveWithdrawal = approveWithdrawal;\nwindow.rejectWithdrawal = rejectWithdrawal;\n"},
	}
	for _, page := range pages {
		data, _, err := p.read(page.source)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		content := strings.ReplaceAll(strings.ReplaceAll(text(data), "window.ApiClient", "ApiClient"), "window.showToast", "showToast")
		// Admin also lives in js/pages; ./api and ./components were wrong in Python.
		content = addImports(content, "import ApiClient from '../api/client.js';", "import { showToast } from '../components/toast.js';") + page.exports
		if err := p.write("js/pages/"+page.name+".js", content, true); err != nil {
			return err
		}
		if err := p.remove(page.source); err != nil {
			return err
		}
	}
	return nil
}

func (p *Plan) htmlFiles() ([]string, error) {
	entries, err := os.ReadDir(p.Root)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".html") {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func (p *Plan) updateCSS() error {
	files, err := p.htmlFiles()
	if err != nil {
		return err
	}
	core := regexp.MustCompile(`<link rel="stylesheet" href="css/tokens\.css">\s*<link rel="stylesheet" href="css/base\.css">\s*<link rel="stylesheet" href="css/components\.css">`)
	pageCSS := regexp.MustCompile(`href="css/(landing|auth|dashboard|admin)\.css"`)
	replacement := "    <link rel=\"stylesheet\" href=\"css/tokens.css\">\n    <link rel=\"stylesheet\" href=\"css/base.css\">\n    <link rel=\"stylesheet\" href=\"css/layout.css\">\n    <link rel=\"stylesheet\" href=\"css/components.css\">\n    <link rel=\"stylesheet\" href=\"css/utilities.css\">"
	for _, file := range files {
		data, _, err := p.read(file)
		if err != nil {
			return err
		}
		content := pageCSS.ReplaceAllString(core.ReplaceAllString(text(data), replacement), `href="css/pages/${1}.css"`)
		if err := p.write(file, content, false); err != nil {
			return err
		}
	}
	return nil
}

func (p *Plan) updateScripts() error {
	files, err := p.htmlFiles()
	if err != nil {
		return err
	}
	old := regexp.MustCompile(`<script src="js/(api|components)\.js"></script>\n?\s*`)
	pageScript := regexp.MustCompile(`<script src="js/([a-zA-Z0-9_]+)\.js"></script>`)
	known := map[string]bool{"auth": true, "dashboard": true, "market": true, "profile": true, "wallet": true, "admin": true}
	for _, file := range files {
		data, _, err := p.read(file)
		if err != nil {
			return err
		}
		content := pageScript.ReplaceAllStringFunc(old.ReplaceAllString(text(data), ""), func(match string) string {
			name := pageScript.FindStringSubmatch(match)[1]
			if !known[name] {
				return match
			}
			return `<script type="module" src="js/pages/` + name + `.js"></script>`
		})
		if err := p.write(file, content, false); err != nil {
			return err
		}
	}
	for _, page := range []string{"dashboard", "market", "profile", "wallet", "admin"} {
		name := "js/pages/" + page + ".js"
		data, _, err := p.read(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		content := text(data)
		prefix := ""
		for _, statement := range []string{"import '../components/sidebar.js';", "import '../components/topbar.js';"} {
			if !strings.Contains(content, statement) {
				prefix += statement + "\n"
			}
		}
		content = prefix + content
		if err := p.write(name, content, false); err != nil {
			return err
		}
	}
	return nil
}

func (p *Plan) validate(change Change) error {
	data, _, err := p.read(change.Path)
	if !change.Existed && errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !change.Existed || !bytes.Equal(data, change.Before) {
		return fmt.Errorf("file changed after preview: %s", change.Path)
	}
	return nil
}

func atomicWrite(target string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".frontend-migrate-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, target)
}

func Apply(p *Plan) (string, error) { return apply(p, atomicWrite) }

func apply(p *Plan, write func(string, []byte, fs.FileMode) error) (string, error) {
	if len(p.Changes) == 0 {
		return "", nil
	}
	lockPath, err := p.safePath(".frontend-migrate.lock")
	if err != nil {
		return "", err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("another migration may be active: %w", err)
	}
	lock.Close()
	defer os.Remove(lockPath)
	for _, change := range p.Changes {
		if err := p.validate(change); err != nil {
			return "", err
		}
	}
	backupRoot, err := p.safePath(".frontend-migrate-backups")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(backupRoot, 0700); err != nil {
		return "", err
	}
	backup, err := os.MkdirTemp(backupRoot, "migration-")
	if err != nil {
		return "", err
	}
	for _, change := range p.Changes {
		if change.Existed {
			if err := atomicWrite(filepath.Join(backup, filepath.FromSlash(change.Path)), change.Before, 0600); err != nil {
				return backup, err
			}
		}
	}
	manifest, err := json.MarshalIndent(p.Changes, "", "  ")
	if err != nil {
		return backup, err
	}
	if err := atomicWrite(filepath.Join(backup, "manifest.json"), manifest, 0600); err != nil {
		return backup, err
	}
	var committed []Change
	rollback := func(cause error) error {
		for i := len(committed) - 1; i >= 0; i-- {
			change := committed[i]
			target, err := p.safePath(change.Path)
			if err == nil {
				if change.Existed {
					err = atomicWrite(target, change.Before, change.Mode)
				} else {
					err = os.Remove(target)
				}
			}
			if err != nil {
				cause = errors.Join(cause, fmt.Errorf("rollback %s: %w", change.Path, err))
			}
		}
		return cause
	}
	// Write destinations first, then retire sources. Preflight or write errors
	// retain the originals; a later error restores every committed file.
	for _, deleting := range []bool{false, true} {
		for _, change := range p.Changes {
			if change.Delete != deleting {
				continue
			}
			if err := p.validate(change); err != nil {
				return backup, rollback(err)
			}
			target, err := p.safePath(change.Path)
			if err != nil {
				return backup, rollback(err)
			}
			if change.Delete {
				err = os.Remove(target)
			} else {
				err = write(target, change.After, change.Mode)
			}
			if err != nil {
				return backup, rollback(err)
			}
			committed = append(committed, change)
		}
	}
	return backup, nil
}
