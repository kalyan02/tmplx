package tmplx

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template/parse"
)

// Package tmpl provides a template engine with inheritance, blocks and includes support.
// It extends Go's html/template package to add template inheritance similar to Django/Jinja2.
//
// Features:
// - Template inheritance with {% extend "base.html" %}
// - Block definitions and overriding
// - Template includes
// - Custom function maps
// - File system abstraction (works with embed.FS)
//
// Example usage:
//
//	engine := tmpl.New(tmpl.Options{
//	    Dir: "templates",
//	    FuncMap: template.FuncMap{
//	        "upper": strings.ToUpper,
//	    },
//	})
//
//	if err := engine.Load(); err != nil {
//	    log.Fatal(err)
//	}
//
//	result, err := engine.Render("pages/home.html", data)

// TemplateEngine manages template loading, caching and rendering with inheritance support.
// It provides a layer on top of html/template to support template inheritance and includes.

type inclCache struct {
	content string
	tmpl    *template.Template
}

type TemplateEngine struct {
	root      string
	fs        fs.FS
	cache     map[string]*template.Template
	loadCache map[string]*template.Template
	inclCache map[string]*inclCache
	funcMap   template.FuncMap
	loaded    bool
	logger    Logger
}

type templateTree struct {
	name     string
	content  string
	extends  string
	blocks   map[string]string
	includes []string
}

// Options holds configuration options for the template engine
type Options struct {
	// Dir specifies the root directory for template files
	Dir string

	// FS provides an optional fs.FS implementation for reading templates
	// If nil, os.DirFS(Dir) will be used
	FS fs.FS

	// FuncMap defines custom template functions
	// Note: 'extend', 'block' and 'include' are reserved function names
	FuncMap template.FuncMap

	// Logger for template operations. If nil, uses a no-op logger
	Logger Logger
}

type Logger interface {
	Infof(format string, args ...interface{})
}

type noopLogger struct{}

func (n *noopLogger) Infof(string, ...interface{}) {}

// New creates a new template engine with the given options.
// If no filesystem is provided in options, it will use os.DirFS with the specified directory.
// If no directory is specified, it uses the current directory.

func New(opts Options) *TemplateEngine {

	// Set up the filesystem
	var filesystem fs.FS
	if opts.FS != nil {
		filesystem = opts.FS
	} else if opts.Dir != "" {
		filesystem = os.DirFS(opts.Dir)
	} else {
		filesystem = os.DirFS(".")
	}

	// Set up logger
	logger := opts.Logger
	if logger == nil {
		logger = &noopLogger{}
	}

	funcMap := template.FuncMap{
		// Core functions that can't be overridden
		"extend": func(name string) (string, error) {
			return "", fmt.Errorf("extend can only be called during template parsing")
		},
		"block": func(name string) (string, error) {
			return "", fmt.Errorf("block can only be called during template parsing")
		},
		"include": func(name string, data interface{}) (string, error) {
			return "", fmt.Errorf("include can only be called during template parsing")
		},
	}

	// Add user-provided functions
	for name, fn := range opts.FuncMap {
		if name != "extend" && name != "include" {
			funcMap[name] = fn
		}
	}

	return &TemplateEngine{
		root:      ".",
		fs:        filesystem,
		cache:     make(map[string]*template.Template),
		loadCache: make(map[string]*template.Template),
		inclCache: make(map[string]*inclCache),
		funcMap:   funcMap,
		logger:    logger,
	}
}

// Load loads all templates from the filesystem into memory.
// This must be called before using the engine for rendering.
// It will parse all .html files and resolve template inheritance.
func (e *TemplateEngine) Load() error {
	if e.loaded {
		return nil
	}

	if err := e.LoadTemplates(); err != nil {
		return fmt.Errorf("failed to load templates: %v", err)
	}

	e.loaded = true
	return nil
}

// NewTemplateEngine creates a new template engine with the given root directory
// and immediately loads all templates. This is a convenience function combining
// New() and Load().
func NewTemplateEngine(root string) (*TemplateEngine, error) {
	e := New(Options{Dir: root})
	return e, e.Load()
}

// AddFuncs adds custom functions to the template engine's function map.
// This will trigger a reload of all templates since the functions might be used in them.
func (e *TemplateEngine) AddFuncs(funcMap template.FuncMap) error {
	// Add all functions to the engine's funcMap
	for name, fn := range funcMap {
		e.funcMap[name] = fn
	}

	// Need to reload templates since functions might be used in them
	return e.LoadTemplates()
}

func (e *TemplateEngine) parseTemplateFile(path string) (*templateTree, error) {

	content, err := fs.ReadFile(e.fs, path)
	if err != nil {
		return nil, err
	}

	tree := &templateTree{
		name:     filepath.Base(path),
		content:  string(content),
		blocks:   make(map[string]string),
		includes: []string{},
	}

	// First do a pre-parse scan for extend directive
	scanner := template.New("").Funcs(e.funcMap)
	parsed, err := scanner.Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("error scanning template %s: %v", path, err)
	}

	// Extract extends directive
	for _, node := range parsed.Tree.Root.Nodes {
		if action, ok := node.(*parse.ActionNode); ok {
			if len(action.Pipe.Cmds) > 0 {
				cmd := action.Pipe.Cmds[0]
				if len(cmd.Args) > 0 {
					if ident, ok := cmd.Args[0].(*parse.IdentifierNode); ok {
						switch ident.Ident {
						case "extend":
							if len(cmd.Args) != 2 {
								return nil, fmt.Errorf("extend requires exactly one argument")
							}
							if str, ok := cmd.Args[1].(*parse.StringNode); ok {
								tree.extends = str.Text
								tree.content = strings.Replace(tree.content, node.String(), "", 1)
							}
						case "include":
							if len(cmd.Args) < 2 {
								return nil, fmt.Errorf("include requires at least one argument")
							}
							if str, ok := cmd.Args[1].(*parse.StringNode); ok {
								tree.includes = append(tree.includes, str.Text)
							}
						}
					}
				}
			}
		}
	}

	// Now create template without extend function
	tmpl := template.New(tree.name).Funcs(e.funcMapWithFuncs(template.FuncMap{
		"block":   func(string, interface{}) (string, error) { return "", nil },
		"include": func(string) (string, error) { return "", nil },
	}))

	// Parse the content after extend directive has been removed
	_, err = tmpl.Parse(tree.content)
	if err != nil {
		return nil, fmt.Errorf("error parsing template %s: %v", path, err)
	}

	return tree, nil
}

func (e *TemplateEngine) funcMapCopy() template.FuncMap {
	funcMap := make(template.FuncMap)
	for k, v := range e.funcMap {
		funcMap[k] = v
	}
	return funcMap
}

func (e *TemplateEngine) funcMapWithFuncs(funcs template.FuncMap) template.FuncMap {
	funcMap := e.funcMapCopy()
	for k, v := range funcs {
		funcMap[k] = v
	}
	return funcMap
}

func (e *TemplateEngine) resolveInheritance(name string, visited map[string]bool) (*template.Template, error) {
	if visited[name] {
		return nil, fmt.Errorf("circular template inheritance detected for %s", name)
	}
	visited[name] = true

	if tmpl, ok := e.loadCache[name]; ok {
		e.logger.Infof("[TMPLX] Returning cached inheritance for %s", name)
		return tmpl, nil
	}

	e.logger.Infof("[TMPLX] Resolving inheritance for %s", name)

	currentPath := filepath.Join(e.root, name)
	tree, err := e.parseTemplateFile(currentPath)
	if err != nil {
		return nil, err
	}

	// If this template extends another, resolve the parent first
	if tree.extends != "" {
		parentPath := tree.extends

		// Resolve the parent template first
		parentTemplate, err := e.resolveInheritance(parentPath, visited)
		if err != nil {
			return nil, fmt.Errorf("error resolving parent template %s: %v", parentPath, err)
		}

		// Create new template with the current name and funcs
		baseTemplate := template.New(tree.name).Funcs(e.funcMap)

		// Parse parent content first - this establishes the base structure
		_, err = baseTemplate.Parse(parentTemplate.Tree.Root.String())
		if err != nil {
			return nil, fmt.Errorf("error parsing parent content: %v", err)
		}

		// Copy all associated templates from parent
		if err := e.copyTemplates(baseTemplate, parentTemplate); err != nil {
			return nil, err
		}

		//DebugTemplate(baseTemplate)

		// Process includes in the current content
		currentContent := removeExtendDirective(tree.content)
		processedContent, includeTmpl, err := e.processIncludes(currentContent, name, make(map[string]bool))
		if err != nil {
			return nil, fmt.Errorf("error processing includes: %v", err)
		}

		// Create temporary template to parse child content
		childTemplate := template.New("temp").Funcs(e.funcMap)
		_, err = childTemplate.Parse(processedContent)
		if err != nil {
			return nil, fmt.Errorf("error parsing child template %s: %v", name, err)
		}

		// Copy all block definitions from includes
		if includeTmpl != nil {
			err = e.copyTemplates(baseTemplate, includeTmpl)
			if err != nil {
				return nil, err
			}
		}

		// Only copy the block definitions from child
		err = e.copyBlockTemplates(baseTemplate, childTemplate)
		if err != nil {
			return nil, err
		}

		//DebugTemplate(baseTemplate)

		_ = baseTemplate
		e.loadCache[name] = baseTemplate
		return baseTemplate, nil

	}

	// For base templates
	baseTemplate := template.New(tree.name).Funcs(e.funcMap)

	// Process includes first
	processedContent, includeTmpl, err := e.processIncludes(tree.content, name, make(map[string]bool))
	if err != nil {
		return nil, fmt.Errorf("error processing includes: %v", err)
	}

	// Copy block definitions from includes first
	if includeTmpl != nil {
		err = e.copyTemplates(baseTemplate, includeTmpl)
		if err != nil {
			return nil, err
		}
	}

	// Parse the current template's content - this will define/override blocks
	// First remove any extend directive from the current template
	_, err = baseTemplate.Parse(processedContent)
	if err != nil {
		return nil, fmt.Errorf("error parsing template %s: %v", name, err)
	}

	//DebugTemplate(baseTemplate)
	_ = baseTemplate

	e.loadCache[name] = baseTemplate
	return baseTemplate, nil
}

func (e *TemplateEngine) copyBlockTemplates(baseTemplate *template.Template, childTemplate *template.Template) error {
	for _, t := range childTemplate.Templates() {
		if t.Name() != "temp" {
			_, err := baseTemplate.AddParseTree(t.Name(), t.Tree)
			if err != nil {
				return fmt.Errorf("error copying block %s: %v", t.Name(), err)
			}
		}
	}
	return nil
}

func (e *TemplateEngine) copyTemplates(baseTemplate *template.Template, includeTmpl *template.Template) error {
	for _, t := range includeTmpl.Templates() {
		if t.Name() != "" && t.Name() != includeTmpl.Name() {
			_, err := baseTemplate.AddParseTree(t.Name(), t.Tree)
			if err != nil {
				return fmt.Errorf("error copying included template %s: %v", t.Name(), err)
			}
		}
	}
	return nil
}

func (e *TemplateEngine) processIncludes(content string, currentFile string, visited map[string]bool) (string, *template.Template, error) {
	if tmpl, ok := e.inclCache[currentFile]; ok {
		e.logger.Infof("[TMPLX] Returning cached include file %s", currentFile)
		return tmpl.content, tmpl.tmpl, nil
	}

	e.logger.Infof("[TMPLX] Processing include file %s", currentFile)

	// Create initial template for collecting block definitions
	collectingTmpl := template.New("").Funcs(e.funcMap)

	// Parse template to find includes
	tmpl := template.New("").Funcs(e.funcMap)

	parsed, err := tmpl.Parse(content)
	if err != nil {
		return "", nil, fmt.Errorf("error parsing template for includes: %v", err)
	}

	processed := content

	// Find all include nodes and process them
	for _, node := range parsed.Tree.Root.Nodes {
		if action, ok := node.(*parse.ActionNode); ok {
			if len(action.Pipe.Cmds) > 0 {
				cmd := action.Pipe.Cmds[0]
				if len(cmd.Args) > 0 {
					if ident, ok := cmd.Args[0].(*parse.IdentifierNode); ok && ident.Ident == "include" {
						if len(cmd.Args) < 2 {
							return "", nil, fmt.Errorf("include requires a template name")
						}
						if str, ok := cmd.Args[1].(*parse.StringNode); ok {
							includePath := str.Text
							if visited[includePath] {
								return "", nil, fmt.Errorf("circular include detected: %s", includePath)
							}

							// Read the included template
							includeFullPath := filepath.Join(e.root, includePath)
							includeContent, err := fs.ReadFile(e.fs, includeFullPath)
							if err != nil {
								return "", nil, fmt.Errorf("error reading include %s: %v", includePath, err)
							}

							// Process nested includes
							visitedCopy := make(map[string]bool)
							for k, v := range visited {
								visitedCopy[k] = v
							}
							visitedCopy[includePath] = true

							processedInclude, includeTmpl, err := e.processIncludes(string(includeContent), includePath, visitedCopy)
							if err != nil {
								return "", nil, fmt.Errorf("error processing nested includes in %s: %v", includePath, err)
							}

							// Copy any block definitions from the included template
							if includeTmpl != nil {
								for _, t := range includeTmpl.Templates() {
									if t.Name() != "" && t.Name() != includeTmpl.Name() {
										_, err = collectingTmpl.AddParseTree(t.Name(), t.Tree)
										if err != nil {
											return "", nil, fmt.Errorf("error copying template %s: %v", t.Name(), err)
										}
									}
								}
							}

							// Replace the include directive with the actual content
							processed = strings.Replace(processed, node.String(), processedInclude, 1)
						}
					}
				}
			}
		}
	}

	// Parse the processed content to get any block definitions
	_, err = collectingTmpl.Parse(processed)
	if err != nil {
		return "", nil, fmt.Errorf("error parsing processed content: %v", err)
	}

	e.inclCache[currentFile] = &inclCache{
		content: processed,
		tmpl:    collectingTmpl,
	}

	return processed, collectingTmpl, nil
}

// Helper function to remove extend directive
func removeExtendDirective(content string) string {
	if idx := strings.Index(content, `{{extend "`); idx != -1 {
		if endIdx := strings.Index(content[idx:], `"}}`); endIdx != -1 {
			endIdx += idx + 3
			return content[:idx] + content[endIdx:]
		}
	}
	return content
}

func (e *TemplateEngine) LoadTemplates() error {
	e.logger.Infof("[TMPLX] Loading templates")
	return fs.WalkDir(e.fs, e.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		relPath, err := filepath.Rel(e.root, path)
		if err != nil {
			return err
		}

		// Resolve template inheritance
		e.logger.Infof("[TMPLX] Processing %s", relPath)
		tmpl, err := e.resolveInheritance(relPath, make(map[string]bool))
		if err != nil {
			return fmt.Errorf("error resolving inheritance for %s: %v", relPath, err)
		}

		e.cache[relPath] = tmpl
		return nil
	})
}

func (e *TemplateEngine) GetTemplate(name string) (*template.Template, error) {
	tmpl, exists := e.cache[name]
	if !exists {
		return nil, fmt.Errorf("template %s not found", name)
	}
	return tmpl, nil
}

func (e *TemplateEngine) MustGetTemplate(name string) *template.Template {
	tmpl, err := e.GetTemplate(name)
	if err != nil {
		panic(err)
	}
	return tmpl
}

func (e *TemplateEngine) renderTo(w io.Writer, name string, data interface{}) error {
	tmpl, exists := e.cache[name]
	if !exists {
		return fmt.Errorf("template %s not found", name)
	}

	// Execute the root template
	err := tmpl.Execute(w, data)
	if err != nil {
		return fmt.Errorf("error rendering template %s: %v", name, err)
	}

	return nil
}

func (e *TemplateEngine) Render(name string, data interface{}) (string, error) {
	var buf strings.Builder
	err := e.renderTo(&buf, name, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (e *TemplateEngine) RenderResponse(w io.Writer, name string, data interface{}) error {
	return e.renderTo(w, name, data)
}

func DebugTemplate(t *template.Template) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Template %q:\n", t.Name()))

	// List all associated templates
	templates := t.Templates()
	for _, tmpl := range templates {
		b.WriteString(fmt.Sprintf("  - %q:\n", tmpl.Name()))
		if tmpl.Tree != nil && tmpl.Tree.Root != nil {
			b.WriteString(fmt.Sprintf("    Content: %s\n", tmpl.Tree.Root.String()))
		}
	}

	fmt.Println(b.String())
	return b.String()
}
