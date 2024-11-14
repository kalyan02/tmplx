package tmplx

import (
	"fmt"
	"io"
)

var (
	DefaultEngine *TemplateEngine
)

// H is a shortcut for map[string]interface{}
type H map[string]interface{}

// Load initializes the default template engine and loads all templates
func Load(opts Options) error {
	DefaultEngine = New(opts)
	if err := DefaultEngine.Load(); err != nil {
		return fmt.Errorf("error loading tmplx engine: %v", err)
	}

	return nil
}

// Render renders a template and returns the output as a string
func Render(name string, data H) (string, error) {
	return DefaultEngine.Render(name, data)
}

// RenderResponse renders a template and writes it to the response writer
func RenderResponse(w io.Writer, name string, data H) error {
	return DefaultEngine.RenderResponse(w, name, data)
}
