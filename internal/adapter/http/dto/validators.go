package dto

import (
"html"
"net/url"
"reflect"
"regexp"
"strings"

"github.com/gin-gonic/gin/binding"
"github.com/go-playground/validator/v10"
)

var safeStringRe = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)

func init() {
if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
_ = v.RegisterValidation("safe_id", validateSafeID)
_ = v.RegisterValidation("safe_url", validateSafeURL)
}
}

// validateSafeID allows alphanumeric, underscore, dash, and dot.
func validateSafeID(fl validator.FieldLevel) bool {
return safeStringRe.MatchString(fl.Field().String())
}

// validateSafeURL accepts only http/https URLs.
func validateSafeURL(fl validator.FieldLevel) bool {
raw := fl.Field().String()
if raw == "" {
return true // optional field; use "required" tag to enforce presence
}
u, err := url.ParseRequestURI(raw)
if err != nil {
return false
}
return u.Scheme == "http" || u.Scheme == "https"
}

// SanitizeStruct trims whitespace and HTML-escapes every exported string
// field (including *string) of a struct pointer.
func SanitizeStruct(v interface{}) {
rv := reflect.ValueOf(v)
if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
return
}
sanitizeFields(rv.Elem())
}

func sanitizeFields(rv reflect.Value) {
for i := 0; i < rv.NumField(); i++ {
f := rv.Field(i)
if !f.CanSet() {
continue
}
switch f.Kind() {
case reflect.String:
f.SetString(sanitize(f.String()))
case reflect.Ptr:
if f.IsNil() {
continue
}
elem := f.Elem()
if elem.Kind() == reflect.String {
s := sanitize(elem.String())
elem.SetString(s)
}
}
}
}

func sanitize(s string) string {
return html.EscapeString(strings.TrimSpace(s))
}
