package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicFilesExcludeDirectoriesDotFilesAndAPIFallback(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STATIC_ROOT", root)
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("public landing"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("not public"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(root, "js"), 0700))
	r := gin.New()
	servePublicFiles(r)
	for _, path := range []string{"/", "/.env", "/js/", "/api/missing", "/%2eenv"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if path == "/" {
			require.Equal(t, 200, w.Code)
			require.Contains(t, w.Body.String(), "public landing")
		} else {
			require.Equal(t, 404, w.Code)
			require.NotContains(t, w.Body.String(), "not public")
			require.NotContains(t, w.Body.String(), "public landing")
		}
	}
}
