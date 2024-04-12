// Package storage contains an extensible interface for providing persistence
// to simple applications and other prefab plugins.
//
// Stores provides simple create, read, update, delete, and list operations.
// Models are represented as structs and should have a `PK() string` method.
//
// Examples:
//
//		plugin.Register(storage.Plugin(memorystore.New()))
//
//	 func (m *MyPlugin) Init(r *plugin.Registry) error {
//	   m.store = r.Get(storage.PluginName)
//	 }
package storage

import "github.com/dpup/prefab/plugin"

// PluginName can be used to query the storage plugin.
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
