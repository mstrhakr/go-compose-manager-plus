package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestScanFailureReason(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("read: %w", os.ErrNotExist), "not_found"},
		{fmt.Errorf("inspect: %w", os.ErrPermission), "permission_denied"},
		{context.Canceled, "canceled"},
		{context.DeadlineExceeded, "deadline_exceeded"},
		{errors.New("private details"), "other"},
		{nil, "other"},
	}
	for _, tt := range tests {
		if got := scanFailureReason(tt.err); got != tt.want {
			t.Errorf("scanFailureReason(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}
