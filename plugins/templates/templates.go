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

// Render a template.
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
		return "", err
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

type TemplateData struct {
	Data   interface{}
	Config map[string]interface{}
}
