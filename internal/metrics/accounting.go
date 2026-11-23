package metrics

import (
	"errors"
	"fmt"
	"math"
	"os"
)

type FileUsage struct {
	Database int64
	WAL      int64
	Shared   int64
	Journal  int64
	Total    int64
}

// FileUsage reports the current logical file lengths, not allocated blocks or
// peak usage. Concurrent SQLite writes can change the sizes during this read.
func (s *Store) FileUsage() (FileUsage, error) {
	var usage FileUsage
	for _, file := range []struct {
		path     string
		required bool
		size     *int64
	}{
		{s.path, true, &usage.Database},
		{s.path + "-wal", false, &usage.WAL},
		{s.path + "-shm", false, &usage.Shared},
		{s.path + "-journal", false, &usage.Journal},
	} {
		info, err := os.Lstat(file.path)
		if errors.Is(err, os.ErrNotExist) && !file.required {
			continue
		}
		if err != nil {
			return FileUsage{}, fmt.Errorf("inspect metrics file %q: %w", file.path, err)
		}
		if !info.Mode().IsRegular() {
			return FileUsage{}, fmt.Errorf("metrics file %q is not regular", file.path)
		}
		*file.size = info.Size()
		if *file.size < 0 || usage.Total > math.MaxInt64-*file.size {
			return FileUsage{}, errors.New("metrics file sizes exceed accounting range")
		}
		usage.Total += *file.size
	}
	return usage, nil
}
