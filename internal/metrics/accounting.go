package metrics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
)

type PageUsage struct {
	PageSize      int64
	Pages         int64
	ReusablePages int64
}

// PageUsage reports SQLite's current logical database pages. Reusable pages
// remain part of the database file; they are not free filesystem space.
func (s *Store) PageUsage(ctx context.Context) (PageUsage, error) {
	var usage PageUsage
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT page_size FROM pragma_page_size),
		(SELECT page_count FROM pragma_page_count),
		(SELECT freelist_count FROM pragma_freelist_count)`).
		Scan(&usage.PageSize, &usage.Pages, &usage.ReusablePages)
	if err != nil {
		return PageUsage{}, fmt.Errorf("inspect metrics database pages: %w", err)
	}
	if usage.PageSize <= 0 || usage.Pages < 0 || usage.ReusablePages < 0 ||
		usage.ReusablePages > usage.Pages || usage.Pages > math.MaxInt64/usage.PageSize {
		return PageUsage{}, errors.New("invalid metrics database page accounting")
	}
	return usage, nil
}

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
