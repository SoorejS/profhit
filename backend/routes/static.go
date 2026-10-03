package routes

import (
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// servePublicFiles receives a dedicated directory containing public assets only.
// It deliberately does not serve directory listings or fall back API failures
// to HTML, which would conceal a broken API contract.
func servePublicFiles(r *gin.Engine) {
	root := os.Getenv("STATIC_ROOT")
	if root == "" {
		return
	}
	fs := http.Dir(root)
	handler := http.FileServer(fs)
	r.NoRoute(func(c *gin.Context) {
		name := path.Clean("/" + c.Request.URL.Path)
		if strings.HasPrefix(name, "/api/") || name == "/api" || (c.Request.Method != "GET" && c.Request.Method != "HEAD") {
			c.JSON(404, gin.H{"error": "Route not found"})
			return
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasPrefix(part, ".") {
				c.Status(404)
				return
			}
		}
		if name == "/" {
			name = "/index.html"
		}
		file, err := fs.Open(name)
		if err != nil {
			c.Status(404)
			return
		}
		info, err := file.Stat()
		file.Close()
		if err != nil || info.IsDir() {
			c.Status(404)
			return
		}
		handler.ServeHTTP(c.Writer, c.Request)
	})
}
