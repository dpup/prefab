// Package storage contains an extensible interface for providing persistence
// to other prefab plugins.
package storage

import "github.com/dpup/prefab/plugin"

// PluginName can be used to query the storage plugin.
//
// Examples:
//
//		plugin.Register(storage.Plugin(memorystore.New()))
//
//	 func (m *MyPlugin) Init(r *plugin.Registry) error {
//	   m.store = r.Get(storage.PluginName)
//	 }
const PluginName = "storage"

// Plugin wraps a storage implementation for registration.
func Plugin(impl Store) plugin.Plugin {
	return &wrapper{Store: impl}
}

type wrapper struct {
	Store
}

func (p *wrapper) Name() string {
	return PluginName
}
