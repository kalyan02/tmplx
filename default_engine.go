package tmplx

import (
	"fmt"
	"io"
)

var (
	DefaultEngine *TemplateEngine
)

// H is a shortcut for map[string]interface{}
type H map[string]any

// Load initializes the default template engine and loads all templates
func Load(opts Options) error {
	DefaultEngine = New(opts)
	if err := DefaultEngine.Load(); err != nil {
		return fmt.Errorf("error loading tmplx engine: %v", err)
	}

	return nil
}

// Render renders a template and returns the output as a string.
// Returns an error if Load has not been called first.
func Render(name string, data H) (string, error) {
	if DefaultEngine == nil {
		return "", fmt.Errorf("tmplx: DefaultEngine not initialized, call Load() first")
	}
	return DefaultEngine.Render(name, data)
}

// RenderResponse renders a template and writes it to the response writer.
// Returns an error if Load has not been called first.
func RenderResponse(w io.Writer, name string, data H) error {
	if DefaultEngine == nil {
		return fmt.Errorf("tmplx: DefaultEngine not initialized, call Load() first")
	}
	return DefaultEngine.RenderResponse(w, name, data)
}
