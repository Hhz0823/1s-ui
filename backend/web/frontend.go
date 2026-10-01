package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Hhz0823/1s-ui/logger"

	"github.com/gin-gonic/gin"
)

// serveFrontend serves the web UI from SUI_FRONTEND_DIR, for installs that
// run no separate web server (OpenWrt, where one process should do it all):
// the UI under the panel path, its runtime config, and the UI's index for
// any other page below that path. Linux installs keep their nginx gateway
// and leave SUI_FRONTEND_DIR unset. The returned handler answers unrouted
// requests it owns and reports whether it did.
func serveFrontend(engine *gin.Engine, webPath string) func(*gin.Context) bool {
	dir := strings.TrimSpace(os.Getenv("SUI_FRONTEND_DIR"))
	if dir == "" {
		return nil
	}
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err != nil {
		logger.Warning("SUI_FRONTEND_DIR has no index.html; the web UI is not served: ", err)
		return nil
	}
	base := "/" + strings.Trim(webPath, "/") + "/"
	if base == "//" {
		base = "/"
	}
	runtime, _ := json.Marshal(map[string]string{"basePath": base, "backendUrl": ""})
	configJS := []byte("window.__SUI_CONFIG__ = Object.freeze(" + string(runtime) + ");\n")
	engine.GET("/.well-known/1s-ui/config.js", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "application/javascript; charset=utf-8", configJS)
	})
	if base != "/" {
		engine.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, base) })
	}
	files := http.StripPrefix(base, http.FileServer(http.Dir(dir)))
	return func(c *gin.Context) bool {
		request := c.Request.URL.Path
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			return false
		}
		// A route for the bare path would make gin redirect base back to it.
		if base != "/" && request == strings.TrimSuffix(base, "/") {
			c.Redirect(http.StatusPermanentRedirect, base)
			return true
		}
		if !strings.HasPrefix(request, base) {
			return false
		}
		relative := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(request, base)), "/")
		for _, api := range []string{"api", "apiv2", "agent"} {
			if relative == api || strings.HasPrefix(relative, api+"/") {
				return false
			}
		}
		if relative != "" {
			if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(relative))); err == nil && !info.IsDir() {
				// Bundles under assets/ carry a build hash in their names.
				if strings.HasPrefix(relative, "assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					c.Header("Cache-Control", "no-cache")
				}
				files.ServeHTTP(c.Writer, c.Request)
				return true
			}
		}
		c.Header("Cache-Control", "no-cache")
		c.File(index)
		return true
	}
}
