package handlers

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed swagger_spec.json
var swaggerSpec []byte

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Inventory API — Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/swagger/doc.json",
      dom_id: "#swagger-ui",
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout",
      deepLinking: true,
      persistAuthorization: true,
    });
  </script>
</body>
</html>`

// SwaggerHandler serves the Swagger UI and the OpenAPI spec.
type SwaggerHandler struct{}

func NewSwaggerHandler() *SwaggerHandler { return &SwaggerHandler{} }

// UI serves the Swagger UI HTML page.
func (h *SwaggerHandler) UI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
}

// Spec serves the raw OpenAPI JSON spec.
func (h *SwaggerHandler) Spec(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", swaggerSpec)
}
