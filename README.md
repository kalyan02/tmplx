
# TMPLX

TMPLX is a Go template engine that extends the standard `html/template` package to provide Django/Jinja2-style template inheritance, blocks, and includes. It maintains the security and familiar syntax of Go templates while adding powerful features for building complex template hierarchies.

## Features

- **Template Inheritance**: Extend base templates using `{{extend "base.html"}}`
- **Block Definitions**: Define and override template blocks
- **Template Includes**: Include other templates with `{{include "partial.html"}}`
- **Custom Functions**: Full support for Go's template.FuncMap
- **Filesystem Abstraction**: Works with regular files or embed.FS
- **Zero External Dependencies**: Built on top of Go's standard library

## Installation

```bash
go get github.com/kalyan02/tmplx
```

## Quick Start

1. Create your templates:

```html
<!-- templates/base.html -->
<!DOCTYPE html>
<html>
<head>
    <title>{{block "title" .}}Default Title{{end}}</title>
</head>
<body>
    {{block "content" .}}
    <p>Default content</p>
    {{end}}
</body>
</html>

<!-- templates/pages/home.html -->
{{extend "base.html"}}

{{block "title" .}}Home Page{{end}}

{{block "content" .}}
    {{include "partials/header.html"}}
    <h1>Welcome, {{.Name}}!</h1>
{{end}}
```

2. Use the template engine:

```go
package main

import (
    "log"
    "strings"
    "github.com/kalyan02/tmplx"
)

func main() {
    // Create a new engine
    engine := tmplx.New(tmplx.Options{
        Dir: "templates",
        FuncMap: template.FuncMap{
            "upper": strings.ToUpper,
        },
    })

    // Load all templates
    if err := engine.Load(); err != nil {
        log.Fatal(err)
    }

    // Render a template
    data := map[string]interface{}{
        "Name": "John",
    }
    
    result, err := engine.Render("pages/home.html", data)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(result)
}
```

## Template Features

### Template Inheritance

Extend base templates using the `extend` directive:

```html
{{extend "layouts/base.html"}}
```

- Must be the first directive in the template
- Child templates can override blocks defined in parent templates
- Supports multiple levels of inheritance

### Blocks

Define reusable blocks that can be overridden by child templates:

```html
{{block "sidebar" .}}
    <!-- default sidebar content -->
{{end}}
```

Override blocks in child templates:

```html
{{block "sidebar" .}}
    <!-- custom sidebar content -->
{{end}}
```

### Includes

Include other templates:

```html
{{include "partials/header.html"}}
```

- Included templates have access to the current context
- Can be used anywhere in templates
- Supports nested includes

## API Reference

### Creating a New Engine

```go
type Options struct {
    Dir     string           // Root directory for templates
    FS      fs.FS           // Optional filesystem (eg. embed.FS)
    FuncMap template.FuncMap // Custom template functions
    Logger  Logger          // Optional logger interface
}

// Create new engine
engine := tmplx.New(Options{
    Dir: "templates",
})
```

### Key Methods

```go
// Load all templates
err := engine.Load()

// Add custom functions
err := engine.AddFuncs(template.FuncMap{
    "myFunc": func() string { return "hello" },
})

// Render a template
result, err := engine.Render("pages/home.html", data)

// Get parsed template
tmpl, err := engine.GetTemplate("pages/home.html")
```

## Using with embed.FS

TMPLX works seamlessly with Go 1.16+ embed.FS:

```go
//go:embed templates/*
var templateFS embed.FS

engine := tmplx.New(tmplx.Options{
    FS: templateFS,
})
```

## Best Practices

1. **Template Organization**:
   - Keep base templates in a `layouts/` directory
   - Put reusable components in `partials/`
   - Organize page templates in `pages/`
   - Use subdirectories for complex projects

2. **Performance**:
   - Call `Load()` once at startup
   - Templates are cached after loading
   - Use `embed.FS` for production deployments

3. **Error Handling**:
   - Always check errors from `Load()` and `Render()`
   - Use the logger interface for debugging template issues

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Acknowledgments

Inspired by Django and Jinja2 template inheritance systems while maintaining Go's template syntax and security model.
