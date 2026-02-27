package handler

import (
"net/http"

"github.com/gin-gonic/gin"
)

// swaggerSpec holds the OpenAPI YAML loaded at startup.
var swaggerSpec []byte

// SetSwaggerSpec sets the OpenAPI specification bytes for serving.
func SetSwaggerSpec(spec []byte) {
swaggerSpec = spec
}

// SwaggerSpec serves the raw OpenAPI YAML.
func SwaggerSpec(c *gin.Context) {
if swaggerSpec == nil {
c.String(http.StatusNotFound, "OpenAPI spec not loaded")
return
}
c.Data(http.StatusOK, "application/x-yaml", swaggerSpec)
}

// SwaggerUI serves an embedded Swagger UI page that loads /swagger/spec.
func SwaggerUI(c *gin.Context) {
html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Secure Payment Gateway - API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/swagger/spec',
      dom_id: '#swagger-ui',
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: 'BaseLayout'
    });
  </script>
</body>
</html>`
c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
