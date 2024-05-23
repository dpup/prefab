// Package storage contains an extensible interface for providing persistence
// to simple applications and other prefab plugins.
//
// Stores provides simple create, read, update, delete, and list operations.
// Models are represented as structs and should have a `PK() string` method.
//
// Examples:
//
//		prefab.Register(storage.Plugin(memorystore.New()))
//
//	 func (m *MyPlugin) Init(r *prefab.Registry) error {
//	   m.store = r.Get(storage.PluginName)
//	 }
package storage

import "github.com/dpup/prefab"

// PluginName can be used to query the storage plugin.
const PluginName = "storage"

// Plugin wraps a storage implementation for registration.
func Plugin(impl Store) prefab.Plugin {
	return &StoragePlugin{Store: impl}
}

// StoragePlugin exposes a Plugin interface for persisting data.
type StoragePlugin struct {
	Store
}

// From prefab.Plugin.
func (p *StoragePlugin) Name() string {
	return PluginName
}

// InitModel can be called by a plugin or application to perform per model
// initialization. Stores that do not implement ModelInitializer should still
// function correctly, but may store data in a shared table.
func (p *StoragePlugin) InitModel(m Model) error {
	if i, ok := p.Store.(ModelInitializer); ok {
		return i.InitModel(m)
	}
	return nil
}
