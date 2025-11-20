// Package templates provides plugins access to Go templates.
//
// Configuration:
// |-----------------------------------|-----------------------|
// | Env                               | JSON                  |
// | ----------------------------------|-----------------------|
// | PF__TEMPLATES__ALWAYSPARSE        | templates.alwaysparse |
// | PF__TEMPLATES__DIRS               | templates.dirs        |
// |-----------------------------------|-----------------------|
package templates

import (
	"bufio"
	"bytes"
	"context"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"google.golang.org/grpc/codes"
)

func init() {
	prefab.RegisterConfigKeys(
		prefab.ConfigKeyInfo{
			Key:         "templates.alwaysParse",
			Description: "Whether to reparse templates on every execution",
			Type:        "bool",
		},
		prefab.ConfigKeyInfo{
			Key:         "templates.dirs",
			Description: "Directories to load templates from",
			Type:        "[]string",
		},
	)
}

// Constant name for identifying the templates plugin.
const PluginName = "templates"

// Plugin returns a new TemplatePlugin.
func Plugin() *TemplatePlugin {
	p := &TemplatePlugin{
		alwaysParse: prefab.Config.Bool("templates.alwaysParse"),
		dirs:        prefab.Config.Strings("templates.dirs"),
	}
	return p
}

// TemplatePlugin exposes utilities for reading and rendering go templates.
type TemplatePlugin struct {
	alwaysParse bool
	dirs        []string
	templates   *template.Template
}

// From prefab.Plugin.
func (p *TemplatePlugin) Name() string {
	return PluginName
}

// From prefab.InitializablePlugin.
func (p *TemplatePlugin) Init(ctx context.Context, r *prefab.Registry) error {
	// Parse templates on initialization.
	return p.parseAll()
}

// Load templates (*.tmpl) contained within the provided directory and all
// sub-directories.
func (p *TemplatePlugin) Load(dirs []string) error {
	p.init()
	p.dirs = append(p.dirs, dirs...)
	for _, dir := range p.dirs {
		if err := p.parse(dir); err != nil {
			return err
		}
	}

	return nil
}

// Render executes a template by name with the provided data.
//
// The data parameter is wrapped in a TemplateData struct before being passed to the
// template. Within templates, access your data fields using .Data.FieldName, not
// .FieldName directly. The Config field provides access to all configuration values.
//
// Example template usage:
//
//	Hello, {{.Data.Name}}!
//	App version: {{.Config.app.version}}
func (p *TemplatePlugin) Render(ctx context.Context, name string, data interface{}) (string, error) {
	if p.alwaysParse {
		if err := p.parseAll(); err != nil {
			return "", err
		}
	}
	if p.templates == nil {
		return "", errors.NewC("no templates have been initialized", codes.Internal)
	}
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	err := p.templates.ExecuteTemplate(w, name, TemplateData{Data: data, Config: prefab.Config.All()})
	if err != nil {
		w.Flush()
		return "", errors.WrapPrefix(err, "template execution failed (hint: data is wrapped, use .Data.FieldName to access fields)", 0)
	}
	w.Flush()

	return b.String(), nil
}

func (p *TemplatePlugin) init() {
	if p.templates == nil || p.alwaysParse {
		p.templates = template.New("").Funcs(template.FuncMap{
			// template functions can be added here.
		})
	}
}

func (p *TemplatePlugin) parseAll() error {
	p.init()
	for _, dir := range p.dirs {
		if err := p.parse(dir); err != nil {
			return err
		}
	}
	return nil
}

func (p *TemplatePlugin) parse(dir string) error {
	return filepath.Walk(dir, func(path string, _ os.FileInfo, _ error) error {
		if strings.HasSuffix(path, ".tmpl") {
			if _, err := p.templates.ParseFiles(path); err != nil {
				return err
			}
		}
		return nil
	})
}

// TemplateData is the wrapper struct passed to all templates during rendering.
// Templates should access the original data via .Data and configuration via .Config.
type TemplateData struct {
	// Data contains the user-provided data passed to Render.
	Data interface{}
	// Config contains all configuration values from prefab.Config.
	Config map[string]interface{}
}
