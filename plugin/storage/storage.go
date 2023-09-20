// Package storage contains an extensible interface for providing persistence
// to other prefab plugins.
package storage

// RegistryKey should be used for registering and fetching the storage plugin.
//
// Examples:
//
//		plugin.Register(storage.RegistryKey, memorystore.New())
//
//	 func (m *MyPlugin) Init(r *plugin.Registry) error {
//	   m.store = r.Get(storage.RegistryKey)
//	 }
//
// TODO: Not a huge fan of the name `RegistryKey` but it is explicit. A possible
// alternative is to use Plugin, Name, or maybe just Key.
type RegistryKey struct{}
