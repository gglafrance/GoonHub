package core

import (
	"fmt"
	"unicode/utf8"

	"goonhub/internal/data"
)

// validScanPath reports whether a scanned file path can be stored. PostgreSQL text columns
// only take valid UTF-8: a file name written in a legacy encoding (e.g. Windows-1252 bytes
// such as 0x96) cannot be recorded as a scene and is skipped with an error of its own.
func validScanPath(path string) bool {
	return utf8.ValidString(path)
}

// sceneBatchCreator is the part of the scene repository the scan batches need.
type sceneBatchCreator interface {
	CreateInBatches(scenes []*data.Scene, batchSize int) error
}

// createScenesResilient inserts the scenes of a scan batch. When the batch insert fails, the
// scenes are inserted one by one so that one bad row does not drop the others: it returns the
// created scenes, the scenes that failed and the first error.
func createScenesResilient(repo sceneBatchCreator, scenes []*data.Scene, batchSize int) (created, failed []*data.Scene, firstErr error) {
	if len(scenes) == 0 {
		return nil, nil, nil
	}
	err := repo.CreateInBatches(scenes, batchSize)
	if err == nil {
		return scenes, nil, nil
	}
	firstErr = fmt.Errorf("batch insert failed, retrying one by one: %w", err)
	for _, sc := range scenes {
		sc.ID = 0 // a failed batch may have left IDs on the structs
		if err := repo.CreateInBatches([]*data.Scene{sc}, 1); err != nil {
			failed = append(failed, sc)
			continue
		}
		created = append(created, sc)
	}
	return created, failed, firstErr
}
