package memorystore

import (
	"testing"

	"github.com/dpup/prefab/storage/storagetests"
)

func TestMemoryStore(t *testing.T) {
	storagetests.Run(t, New)
}
