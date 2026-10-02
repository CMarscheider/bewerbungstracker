package httpapi

import "github.com/getkin/kin-openapi/openapi3"

// kin-openapi prüft das Format "uuid" nur, wenn ein Validator registriert ist.
func init() {
	openapi3.DefineStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(openapi3.FormatOfStringForUUIDOfRFC4122))
}
