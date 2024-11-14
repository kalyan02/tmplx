package tmplx

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func setupTestTemplates(t testing.TB) (string, func()) {
	// Create a temporary directory for test templates
	tempDir, err := os.MkdirTemp("", "template-tests-*")
	if err != nil {
		t.Fatal(err)
	}

	// Create directory structure
	dirs := []string{
		"layouts",
		"layouts/admin",
		"pages",
		"pages/admin",
		"partials",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Cleanup function
	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func writeTemplate(t testing.TB, dir, path, content string) {
	fullPath := filepath.Join(dir, path)
	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSimpleTemplate(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Modify the simple template to use the block structure
	writeTemplate(t, tempDir, "pages/simple.html", `{{block "content" .}}
    <h1>{{.Title}}</h1>
    <p>{{.Content}}</p>
{{end}}`)

	engine, err := NewTemplateEngine(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := map[string]interface{}{
		"Title":   "Hello",
		"Content": "World",
	}

	result, err := engine.Render("pages/simple.html", data)
	if err != nil {
		t.Fatal(err)
	}

	expectedParts := []string{"<h1>Hello</h1>", "<p>World</p>"}
	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected result to contain %q, got %q", part, result)
		}
	}
}

func TestTemplateInheritance(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Create base layout
	writeTemplate(t, tempDir, "layouts/base.html", `
		<!DOCTYPE html>
		<html>
		<head>
			<title>{{block "title" .}}Default Title{{end}}</title>
		</head>
		<body>
			{{block "content" .}}
				Default Content
			{{end}}
		</body>
		</html>
	`)

	// Create page extending base
	writeTemplate(t, tempDir, "pages/child.html", `
		{{extend "layouts/base.html"}}

		{{block "title" .}}{{.Title}}{{end}}

		{{block "content" .}}
			<h1>{{.Title}}</h1>
			<div>{{.Content}}</div>
		{{end}}
	`)

	engine, err := NewTemplateEngine(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := map[string]interface{}{
		"Title":   "Page Title",
		"Content": "Page Content",
	}

	result, err := engine.Render("pages/child.html", data)
	if err != nil {
		t.Fatal(err)
	}

	expectedParts := []string{
		"<title>Page Title</title>",
		"<h1>Page Title</h1>",
		"<div>Page Content</div>",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected result to contain %q, got %q", part, result)
		}
	}
}

func TestNestedBlocks(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Base layout - define the main template structure
	writeTemplate(t, tempDir, "layouts/base.html", `<!DOCTYPE html>
<html>
<head>
    {{block "head" .}}
        <title>{{block "title" .}}Default Title{{end}}</title>
        {{block "meta" .}}
            <meta name="description" content="Default description">
        {{end}}
    {{end}}
</head>
<body>
    {{block "body" .}}
        <main>
            {{block "content" .}}Default Content{{end}}
        </main>
        <aside>
            {{block "sidebar" .}}Default Sidebar{{end}}
        </aside>
    {{end}}
</body>
</html>`)

	// Page extending base - note how we're only defining the blocks we want to override
	writeTemplate(t, tempDir, "pages/nested.html", `{{extend "layouts/base.html"}}

{{define "title"}}{{.Title}}{{end}}

{{define "content"}}
    <h1>{{.Title}}</h1>
    {{block "subcontent" .}}
        <p>{{.Content}}</p>
    {{end}}
{{end}}

{{define "sidebar"}}
    <nav>
        {{range .Menu}}
            <a href="{{.URL}}">{{.Text}}</a>
        {{end}}
    </nav>
{{end}}`)

	// Rest of the test remains the same...
}

func TestCircularInheritance(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Create templates with circular inheritance
	writeTemplate(t, tempDir, "pages/a.html", `
		{{extend "pages/b.html"}}
		{{block "content" .}}A content{{end}}
	`)

	writeTemplate(t, tempDir, "pages/b.html", `
		{{extend "pages/a.html"}}
		{{block "content" .}}B content{{end}}
	`)

	_, err := NewTemplateEngine(tempDir)
	if err == nil {
		t.Error("Expected error for circular inheritance, got nil")
	}
}

func TestContextPassing(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Create base layout
	writeTemplate(t, tempDir, "layouts/base.html", `
		{{block "content" .}}{{end}}
	`)

	// Create nested template with context usage
	writeTemplate(t, tempDir, "pages/context.html", `
		{{extend "layouts/base.html"}}

		{{block "content" .}}
			{{with .User}}
				<h1>{{.Name}}</h1>
				{{block "user-details" .}}
					<div>
						<p>Email: {{.Email}}</p>
						{{with .Profile}}
							<p>Bio: {{.Bio}}</p>
						{{end}}
					</div>
				{{end}}
			{{end}}

			{{range .Items}}
				<div>{{.Name}}: {{.Value}}</div>
			{{end}}
		{{end}}
	`)

	engine, err := NewTemplateEngine(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := map[string]interface{}{
		"User": map[string]interface{}{
			"Name":  "John Doe",
			"Email": "john@example.com",
			"Profile": map[string]interface{}{
				"Bio": "Test bio",
			},
		},
		"Items": []struct {
			Name  string
			Value string
		}{
			{"Item1", "Value1"},
			{"Item2", "Value2"},
		},
	}

	result, err := engine.Render("pages/context.html", data)
	if err != nil {
		t.Fatal(err)
	}

	expectedParts := []string{
		"<h1>John Doe</h1>",
		"<p>Email: john@example.com</p>",
		"<p>Bio: Test bio</p>",
		"<div>Item1: Value1</div>",
		"<div>Item2: Value2</div>",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected result to contain %q, got %q", part, result)
		}
	}
}

func TestInvalidTemplate(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Create template with syntax error
	writeTemplate(t, tempDir, "pages/invalid.html", `
		{{extend "layouts/base.html"}}
		{{block "content" .}}
			{{.Unclosed}
		{{end}}
	`)

	_, err := NewTemplateEngine(tempDir)
	if err == nil {
		t.Error("Expected error for invalid template syntax, got nil")
	}
}

func TestMissingTemplate(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine, err := NewTemplateEngine(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Render("pages/nonexistent.html", nil)
	if err == nil {
		t.Error("Expected error for missing template, got nil")
	}
}

func TestTemplateIncludes(t *testing.T) {

	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Create shared partial template
	writeTemplate(t, tempDir, "partials/header.html", `
		<header>
			<h1>{{.Title}}</h1>
			<nav>
				{{range .NavItems}}
					<a href="{{.URL}}">{{.Name}}</a>
				{{end}}
			</nav>
		</header>
	`)

	// Create shared footer
	writeTemplate(t, tempDir, "partials/footer.html", `
		<footer>
			<p>Copyright © {{.Year}} {{.Company}}</p>
		</footer>
	`)

	// Create base layout that includes header and footer
	writeTemplate(t, tempDir, "layouts/base.html", `
		<!DOCTYPE html>
		<html>
		<head>
			<title>{{.Title}}</title>
		</head>
		<body>
			{{include "partials/header.html" .}}
			{{block "content" .}}{{end}}
			{{include "partials/footer.html" .}}
		</body>
		</html>
	`)

	// Create page using the base layout
	writeTemplate(t, tempDir, "pages/home.html", `
		{{extend "layouts/base.html"}}

		{{block "content" .}}
			<main>
				<h2>Welcome to {{.Title}}</h2>
				<div>{{.Content}}</div>
			</main>
		{{end}}
	`)

	engine, err := NewTemplateEngine(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := map[string]interface{}{
		"Title": "My Website",
		"NavItems": []struct {
			Name string
			URL  string
		}{
			{"Home", "/"},
			{"About", "/about"},
			{"Contact", "/contact"},
		},
		"Content": "Welcome to our site!",
		"Year":    "2024",
		"Company": "Example Corp",
	}

	result, err := engine.Render("pages/home.html", data)
	if err != nil {
		t.Fatal(err)
	}

	expectedParts := []string{
		"<!DOCTYPE html>",
		"<title>My Website</title>",
		"<header>",
		"<h1>My Website</h1>",
		`<a href="/">Home</a>`,
		`<a href="/about">About</a>`,
		`<a href="/contact">Contact</a>`,
		"<h2>Welcome to My Website</h2>",
		"<div>Welcome to our site!</div>",
		"<footer>",
		"<p>Copyright © 2024 Example Corp</p>",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected result to contain %q, got %q", part, result)
		}
	}
}
func TestBadIncludes(t *testing.T) {

	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	engine, err := NewTemplateEngine(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := map[string]interface{}{}

	// Test error cases
	writeTemplate(t, tempDir, "pages/bad_include.html", `
		{{include "partials/nonexistent.html" .}}
	`)

	_, err = engine.Render("pages/bad_include.html", data)
	if err == nil {
		t.Error("Expected error for nonexistent include, got nil")
	}
}

func TestNestedIncludes(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Test nested includes
	writeTemplate(t, tempDir, "partials/nav.html", `
		<nav>
			{{range .NavItems}}
				<a href="{{.URL}}">{{.Name}}</a>
			{{end}}
		</nav>
	`)

	writeTemplate(t, tempDir, "partials/header_with_nav.html", `
		<header>
			<h1>{{.Title}}</h1>
			{{include "partials/nav.html" .}}
		</header>
	`)

	writeTemplate(t, tempDir, "pages/nested_includes.html", `
		{{include "partials/header_with_nav.html" .}}
		<main>{{.Content}}</main>
	`)

	data := map[string]interface{}{
		"Title": "My Website",
		"NavItems": []struct {
			Name string
			URL  string
		}{
			{"Home", "/"},
			{"About", "/about"},
			{"Contact", "/contact"},
		},
		"Content": "Welcome to our site!",
	}

	engine, err := NewTemplateEngine(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.Render("pages/nested_includes.html", data)
	if err != nil {
		t.Fatal(err)
	}

	nestedExpectedParts := []string{
		"<header>",
		"<h1>My Website</h1>",
		"<nav>",
		`<a href="/">Home</a>`,
		`<a href="/about">About</a>`,
		`<a href="/contact">Contact</a>`,
		"</nav>",
		"</header>",
		"<main>Welcome to our site!</main>",
	}

	for _, part := range nestedExpectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected nested include result to contain %q, got %q", part, result)
		}
	}
}

func TestTemplateFuncMap(t *testing.T) {
	tempDir, cleanup := setupTestTemplates(t)
	defer cleanup()

	// Create template using custom functions
	writeTemplate(t, tempDir, "pages/with_funcs.html", `
        {{extend "layouts/base.html"}}

        {{block "content" .}}
            <h1>{{upper .Title}}</h1>
            <p>{{repeat "Hello" 3}}</p>
            {{with .User}}
                <div>{{formatName .FirstName .LastName}}</div>
            {{end}}
            <div>{{add 5 3}}</div>
        {{end}}
    `)

	writeTemplate(t, tempDir, "layouts/base.html", `
        <!DOCTYPE html>
        <html>
        <head>
            <title>{{block "title" .}}Default Title{{end}}</title>
        </head>
        <body>
            {{block "content" .}}Default Content{{end}}
        </body>
        </html>
    `)

	// Create engine with custom functions
	engine := New(Options{
		Dir: tempDir,
		FuncMap: template.FuncMap{
			"upper": strings.ToUpper,
			"repeat": func(s string, n int) string {
				return strings.Repeat(s+" ", n)
			},
			"formatName": func(first, last string) string {
				return fmt.Sprintf("%s %s", first, last)
			},
			"add": func(a, b int) int {
				return a + b
			},
		},
	})

	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}

	data := map[string]interface{}{
		"Title": "My Page",
		"User": map[string]interface{}{
			"FirstName": "John",
			"LastName":  "Doe",
		},
	}

	result, err := engine.Render("pages/with_funcs.html", data)
	if err != nil {
		t.Fatal(err)
	}

	expectedParts := []string{
		"<h1>MY PAGE</h1>",
		"<p>Hello Hello Hello </p>",
		"<div>John Doe</div>",
		"<div>8</div>",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected result to contain %q, got %q", part, result)
		}
	}
}

func TestTemplateEmbedFS(t *testing.T) {
	// Create a testing filesystem
	fsys := fstest.MapFS{
		"layouts/base.html": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html>
                    <html>
                    <head>
                        <title>{{block "title" .}}Default Title{{end}}</title>
                    </head>
                    <body>
                        {{block "content" .}}Default Content{{end}}
                    </body>
                    </html>`),
		},
		"pages/home.html": &fstest.MapFile{
			Data: []byte(`
                    {{extend "layouts/base.html"}}
                    {{block "content" .}}
                        <h1>{{upper .Title}}</h1>
                    {{end}}`),
		},
	}

	// Create engine with embed.FS
	engine := New(Options{
		FS: fsys,
		FuncMap: template.FuncMap{
			"upper": strings.ToUpper,
		},
	})

	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}

	// Test rendering
	result, err := engine.Render("pages/home.html", map[string]interface{}{
		"Title": "Test",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "TEST") {
		t.Errorf("Expected result to contain uppercase title")
	}
}

func BenchmarkTemplateEngine(b *testing.B) {
	tempDir, cleanup := setupTestTemplates(b)
	defer cleanup()

	// Create base layout for our template engine
	writeTemplate(b, tempDir, "layouts/base.html", `
        <!DOCTYPE html>
        <html>
        <head>
            <title>{{block "title" .}}Default Title{{end}}</title>
        </head>
        <body>
            {{block "header" .}}
                <header>
                    <h1>{{.Title}}</h1>
                    <nav>
                        {{range .NavItems}}
                            <a href="{{.URL}}">{{.Name}}</a>
                        {{end}}
                    </nav>
                </header>
            {{end}}
            {{block "content" .}}Default Content{{end}}
            {{block "footer" .}}
                <footer>&copy; {{.Year}}</footer>
            {{end}}
        </body>
        </html>
    `)

	// Create a page template for our template engine
	writeTemplate(b, tempDir, "pages/home.html", `
        {{extend "layouts/base.html"}}

        {{block "title" .}}{{.Title}} - Home{{end}}

        {{block "content" .}}
            <main>
                <h2>Welcome to {{upper .Title}}</h2>
                <div class="content">
                    {{.Content}}
                </div>
                {{range .Items}}
                    <div class="item">
                        <h3>{{.Name}}</h3>
                        <p>{{.Description}}</p>
                    </div>
                {{end}}
            </main>
        {{end}}
    `)

	vanillaTempDir, vanillaCleanup := setupTestTemplates(b)
	defer vanillaCleanup()

	// Create equivalent vanilla templates (without extend directive, using define)
	writeTemplate(b, vanillaTempDir, "home.html", `
        {{define "title"}}{{.Title}} - Home{{end}}

        {{define "content"}}
            <main>
                <h2>Welcome to {{upper .Title}}</h2>
                <div class="content">
                    {{.Content}}
                </div>
                {{range .Items}}
                    <div class="item">
                        <h3>{{.Name}}</h3>
                        <p>{{.Description}}</p>
                    </div>
                {{end}}
            </main>
        {{end}}

        {{template "base" .}}
    `)

	writeTemplate(b, vanillaTempDir, "base.html", `
        {{define "base"}}
        <!DOCTYPE html>
        <html>
        <head>
            <title>{{template "title" .}}</title>
        </head>
        <body>
            {{block "header" .}}
                <header>
                    <h1>{{.Title}}</h1>
                    <nav>
                        {{range .NavItems}}
                            <a href="{{.URL}}">{{.Name}}</a>
                        {{end}}
                    </nav>
                </header>
            {{end}}
            {{template "content" .}}
            {{block "footer" .}}
                <footer>&copy; {{.Year}}</footer>
            {{end}}
        </body>
        </html>
        {{end}}
    `)

	// Prepare test data
	data := map[string]interface{}{
		"Title": "My Website",
		"Year":  "2024",
		"NavItems": []struct {
			Name string
			URL  string
		}{
			{"Home", "/"},
			{"About", "/about"},
			{"Contact", "/contact"},
		},
		"Content": "Welcome to our website!",
		"Items": []struct {
			Name        string
			Description string
		}{
			{"Item 1", "Description 1"},
			{"Item 2", "Description 2"},
			{"Item 3", "Description 3"},
		},
	}

	// Benchmark Template Engine
	b.Run("TemplateEngine", func(b *testing.B) {
		engine := New(Options{
			Dir: tempDir,
			FuncMap: template.FuncMap{
				"upper": strings.ToUpper,
			},
		})

		if err := engine.Load(); err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := engine.Render("pages/home.html", data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Benchmark vanilla Go templates
	b.Run("VanillaTemplates", func(b *testing.B) {
		// Create template with functions
		tmpl := template.New("").Funcs(template.FuncMap{
			"upper": strings.ToUpper,
		})

		// Parse all templates
		if _, err := tmpl.ParseFiles(
			filepath.Join(vanillaTempDir, "base.html"),
			filepath.Join(vanillaTempDir, "home.html"),
		); err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf strings.Builder
			err := tmpl.ExecuteTemplate(&buf, "base", data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
