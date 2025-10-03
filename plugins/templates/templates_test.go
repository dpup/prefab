package templates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dpup/prefab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlugin(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		p := Plugin()
		require.NotNil(t, p)
		assert.Equal(t, PluginName, p.Name())
		// Default config may have templates.dirs set, so just check it's not nil
		assert.NotNil(t, p.dirs)
	})

	t.Run("WithConfig", func(t *testing.T) {
		// Save and restore config
		oldAlwaysParse := prefab.Config.Bool("templates.alwaysParse")
		oldDirs := prefab.Config.Strings("templates.dirs")
		defer func() {
			if oldAlwaysParse {
				prefab.Config.Set("templates.alwaysParse", oldAlwaysParse)
			} else {
				prefab.Config.Delete("templates.alwaysParse")
			}
			if len(oldDirs) > 0 {
				prefab.Config.Set("templates.dirs", oldDirs)
			} else {
				prefab.Config.Delete("templates.dirs")
			}
		}()

		prefab.Config.Set("templates.alwaysParse", true)
		prefab.Config.Set("templates.dirs", []string{"/tmp/test1", "/tmp/test2"})

		p := Plugin()
		assert.True(t, p.alwaysParse)
		assert.Equal(t, []string{"/tmp/test1", "/tmp/test2"}, p.dirs)
	})
}

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()

	// Create test template files
	tmpl1 := filepath.Join(tempDir, "test1.tmpl")
	err := os.WriteFile(tmpl1, []byte("Hello {{.Data}}"), 0644)
	require.NoError(t, err)

	tmpl2 := filepath.Join(tempDir, "test2.tmpl")
	err = os.WriteFile(tmpl2, []byte("Goodbye {{.Data}}"), 0644)
	require.NoError(t, err)

	// Create a non-template file to ensure it's ignored
	nonTmpl := filepath.Join(tempDir, "notatemplate.txt")
	err = os.WriteFile(nonTmpl, []byte("Should be ignored"), 0644)
	require.NoError(t, err)

	t.Run("LoadSingleDir", func(t *testing.T) {
		p := Plugin()
		err := p.Load([]string{tempDir})
		require.NoError(t, err)
		assert.Contains(t, p.dirs, tempDir)
	})

	t.Run("LoadMultipleDirs", func(t *testing.T) {
		p := &TemplatePlugin{} // Use empty plugin to avoid default dirs
		err := p.Load([]string{tempDir, tempDir})
		require.NoError(t, err)
		assert.Len(t, p.dirs, 2)
	})
}

func TestRender(t *testing.T) {
	tempDir := t.TempDir()

	// Create test template
	tmplFile := filepath.Join(tempDir, "greeting.tmpl")
	err := os.WriteFile(tmplFile, []byte("Hello {{.Data}}!"), 0644)
	require.NoError(t, err)

	ctx := t.Context()

	t.Run("RenderSuccess", func(t *testing.T) {
		p := Plugin()
		err := p.Load([]string{tempDir})
		require.NoError(t, err)

		result, err := p.Render(ctx, "greeting.tmpl", "World")
		require.NoError(t, err)
		assert.Equal(t, "Hello World!", result)
	})

	t.Run("RenderWithData", func(t *testing.T) {
		// Create template that uses data fields
		tmplFile := filepath.Join(tempDir, "struct.tmpl")
		err := os.WriteFile(tmplFile, []byte("{{.Data.Name}} is {{.Data.Age}} years old"), 0644)
		require.NoError(t, err)

		p := Plugin()
		err = p.Load([]string{tempDir})
		require.NoError(t, err)

		type Person struct {
			Name string
			Age  int
		}

		result, err := p.Render(ctx, "struct.tmpl", Person{Name: "Alice", Age: 30})
		require.NoError(t, err)
		assert.Equal(t, "Alice is 30 years old", result)
	})

	t.Run("RenderMissingTemplate", func(t *testing.T) {
		p := Plugin()
		err := p.Load([]string{tempDir})
		require.NoError(t, err)

		_, err = p.Render(ctx, "nonexistent.tmpl", "data")
		require.Error(t, err)
	})

	t.Run("RenderNotInitialized", func(t *testing.T) {
		p := &TemplatePlugin{} // Empty plugin with nil templates
		// Don't load any templates or call init
		_, err := p.Render(ctx, "test.tmpl", "data")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no templates have been initialized")
	})

	t.Run("RenderWithConfig", func(t *testing.T) {
		// Create template that uses config
		tmplFile := filepath.Join(tempDir, "config.tmpl")
		err := os.WriteFile(tmplFile, []byte("App: {{index .Config \"name\"}}"), 0644)
		require.NoError(t, err)

		p := Plugin()
		err = p.Load([]string{tempDir})
		require.NoError(t, err)

		result, err := p.Render(ctx, "config.tmpl", nil)
		require.NoError(t, err)
		// The config should have a name field from prefab's default config
		assert.Contains(t, result, "App:")
	})
}

func TestAlwaysParse(t *testing.T) {
	tempDir := t.TempDir()

	// Create initial template
	tmplFile := filepath.Join(tempDir, "dynamic.tmpl")
	err := os.WriteFile(tmplFile, []byte("Version 1: {{.Data}}"), 0644)
	require.NoError(t, err)

	ctx := t.Context()

	t.Run("AlwaysParseEnabled", func(t *testing.T) {
		// Save and restore config
		oldAlwaysParse := prefab.Config.Bool("templates.alwaysParse")
		defer func() {
			if oldAlwaysParse {
				prefab.Config.Set("templates.alwaysParse", oldAlwaysParse)
			} else {
				prefab.Config.Delete("templates.alwaysParse")
			}
		}()

		prefab.Config.Set("templates.alwaysParse", true)

		p := Plugin()
		assert.True(t, p.alwaysParse)
		err := p.Load([]string{tempDir})
		require.NoError(t, err)

		result, err := p.Render(ctx, "dynamic.tmpl", "Test")
		require.NoError(t, err)
		assert.Equal(t, "Version 1: Test", result)

		// Update the template file
		err = os.WriteFile(tmplFile, []byte("Version 2: {{.Data}}"), 0644)
		require.NoError(t, err)

		// With alwaysParse, should see the updated version
		result, err = p.Render(ctx, "dynamic.tmpl", "Test")
		require.NoError(t, err)
		assert.Equal(t, "Version 2: Test", result)
	})

	t.Run("AlwaysParseDisabled", func(t *testing.T) {
		// Reset template to version 1
		err := os.WriteFile(tmplFile, []byte("Version 1: {{.Data}}"), 0644)
		require.NoError(t, err)

		p := &TemplatePlugin{alwaysParse: false}
		err = p.Load([]string{tempDir})
		require.NoError(t, err)

		result, err := p.Render(ctx, "dynamic.tmpl", "Test")
		require.NoError(t, err)
		assert.Equal(t, "Version 1: Test", result)

		// Update the template file
		err = os.WriteFile(tmplFile, []byte("Version 2: {{.Data}}"), 0644)
		require.NoError(t, err)

		// Without alwaysParse, should still see the old cached version
		result, err = p.Render(ctx, "dynamic.tmpl", "Test")
		require.NoError(t, err)
		assert.Equal(t, "Version 1: Test", result)
	})
}

func TestInit(t *testing.T) {
	tempDir := t.TempDir()

	// Create test template
	tmplFile := filepath.Join(tempDir, "init.tmpl")
	err := os.WriteFile(tmplFile, []byte("Initialized: {{.Data}}"), 0644)
	require.NoError(t, err)

	t.Run("InitWithDirs", func(t *testing.T) {
		// Save and restore config
		oldDirs := prefab.Config.Strings("templates.dirs")
		defer func() {
			if len(oldDirs) > 0 {
				prefab.Config.Set("templates.dirs", oldDirs)
			} else {
				prefab.Config.Delete("templates.dirs")
			}
		}()

		prefab.Config.Set("templates.dirs", []string{tempDir})

		p := Plugin()
		ctx := t.Context()
		err := p.Init(ctx, &prefab.Registry{})
		require.NoError(t, err)

		// Should be able to render without explicitly calling Load
		result, err := p.Render(ctx, "init.tmpl", "Success")
		require.NoError(t, err)
		assert.Equal(t, "Initialized: Success", result)
	})

	t.Run("InitWithNoDirs", func(t *testing.T) {
		p := &TemplatePlugin{}
		ctx := t.Context()
		err := p.Init(ctx, &prefab.Registry{})
		require.NoError(t, err)
	})
}

func TestParseErrors(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("InvalidTemplateSyntax", func(t *testing.T) {
		// Create template with invalid syntax
		tmplFile := filepath.Join(tempDir, "invalid.tmpl")
		err := os.WriteFile(tmplFile, []byte("{{.Data"), 0644) // Missing closing braces
		require.NoError(t, err)

		p := Plugin()
		err = p.Load([]string{tempDir})
		require.Error(t, err)
	})
}

func TestSubdirectories(t *testing.T) {
	tempDir := t.TempDir()

	// Create nested directory structure
	subdir := filepath.Join(tempDir, "subdir")
	err := os.MkdirAll(subdir, 0755)
	require.NoError(t, err)

	// Create templates in subdirectory
	tmplFile := filepath.Join(subdir, "nested.tmpl")
	err = os.WriteFile(tmplFile, []byte("Nested: {{.Data}}"), 0644)
	require.NoError(t, err)

	t.Run("LoadSubdirectories", func(t *testing.T) {
		p := Plugin()
		err := p.Load([]string{tempDir})
		require.NoError(t, err)

		ctx := t.Context()
		result, err := p.Render(ctx, "nested.tmpl", "Success")
		require.NoError(t, err)
		assert.Equal(t, "Nested: Success", result)
	})
}
