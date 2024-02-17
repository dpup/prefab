// Package templates provides plugins access to Go templates.
//
// Configuration:
// |-----------------------|-----------------------|
// | Env                   | JSON                  |
// | ----------------------|-----------------------|
// | TEMPLATES_ALWAYSPARSE | templates.alwaysparse |
// | TEMPLATES_DIRS        | templates.dirs        |
// |-----------------------|-----------------------|
package templates

import (
	"bufio"
	"bytes"
	"context"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpup/prefab/plugin"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Constant name for identifying the templates plugin.
const PluginName = "templates"

// Plugin returns a new TemplatePlugin.
func Plugin() *TemplatePlugin {
	p := &TemplatePlugin{
		alwaysParse: viper.GetBool("templates.alwaysparse"),
		dirs:        viper.GetStringSlice("templates.dirs"),
	}
	return p
}

// TemplatePlugin exposes utilities for reading and rendering go templates.
type TemplatePlugin struct {
	alwaysParse bool
	dirs        []string
	templates   *template.Template
}

// From plugin.Plugin
func (p *TemplatePlugin) Name() string {
	return PluginName
}

// From plugin.InitializablePlugin
func (p *TemplatePlugin) Init(ctx context.Context, r *plugin.Registry) error {
	// TODO: Load templates proactively?
	return nil
}

// Load templates (*.tmpl) contained within the provided directory and all
// sub-directories.
func (r *TemplatePlugin) Load(dirs []string) error {
	r.init()
	r.dirs = append(r.dirs, dirs...)
	for _, dir := range r.dirs {
		if err := r.parse(dir); err != nil {
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
		return "", status.Error(codes.Internal, "no templates have been initialized")
	}
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	err := p.templates.ExecuteTemplate(w, name, TemplateData{Data: data, Config: viper.AllSettings()})
	if err != nil {
		w.Flush()
		return "", err
	}
	w.Flush()

	return b.String(), nil
}

func (r *TemplatePlugin) init() {
	if r.templates == nil || r.alwaysParse {
		r.templates = template.New("").Funcs(template.FuncMap{
			// template functions can be added here.
		})
	}
}

func (r *TemplatePlugin) parseAll() error {
	r.init()
	for _, dir := range r.dirs {
		if err := r.parse(dir); err != nil {
			return err
		}
	}
	return nil
}

func (r *TemplatePlugin) parse(dir string) error {
	return filepath.Walk(dir, func(path string, _ os.FileInfo, _ error) error {
		if strings.HasSuffix(path, ".tmpl") {
			if _, err := r.templates.ParseFiles(path); err != nil {
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
