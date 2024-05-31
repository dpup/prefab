package memstore

import (
	"testing"

	"github.com/dpup/prefab/plugins/storage/storagetests"
)

func TestMemoryStore(t *testing.T) {
	storagetests.Run(t, New)
}
