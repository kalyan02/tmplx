package tmplx

import (
	"fmt"
	"net/http"
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
func Render(w http.ResponseWriter, name string, data H) (string, error) {
	return DefaultEngine.Render(name, data)
}

// RenderResponse renders a template and writes it to the response writer
func RenderResponse(w http.ResponseWriter, name string, data H) error {
	out, err := DefaultEngine.Render(name, data)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(out))
	return err
}
