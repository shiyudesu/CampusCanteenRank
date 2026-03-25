package router

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.json
var openAPIDocJSON []byte

const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>CampusCanteenRank API Docs</title>
  <style>
    body { font-family: sans-serif; margin: 2rem; line-height: 1.6; }
    a { color: #0a58ca; }
    code { background: #f2f4f7; padding: 0.1rem 0.3rem; border-radius: 4px; }
  </style>
</head>
<body>
  <h1>CampusCanteenRank API Docs</h1>
  <p>OpenAPI document is served locally to avoid external CDN dependencies.</p>
  <p>JSON spec: <a href="/swagger/doc.json">/swagger/doc.json</a></p>
  <p>Import this URL into your local API tool (for example Postman/Insomnia/Swagger Editor): <code>/swagger/doc.json</code></p>
</body>
</html>`

func registerSwaggerRoutes(r *gin.Engine) {
	r.GET("/swagger", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerUIHTML))
	})
	r.GET("/swagger/doc.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", openAPIDocJSON)
	})
}
