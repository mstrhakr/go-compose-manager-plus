package discovery

import "testing"

func TestMissingInterpolation(t *testing.T) {
	values := map[string]string{"SET": "yes", "EMPTY": ""}
	lookup := func(key string) (string, bool) { value, ok := values[key]; return value, ok }
	for _, test := range []struct {
		input string
		want  bool
	}{
		{"${REPO:-example}:${TAG:-latest}", false},
		{"${FIRST:-${SECOND:-fallback}}", false},
		{"${FIRST:-${SECOND}}", true},
		{"${SET:-${MISSING}}", false},
		{"${SET:+${MISSING}}", true},
		{"${EMPTY:+${MISSING}}", false},
		{"${EMPTY:-${MISSING}}", true},
		{"${EMPTY-${MISSING}}", false},
		{"${MISSING:-x}:${MISSING}", true},
		{"$$MISSING", false},
		{"$SET:$MISSING", true},
		{"${MISSING:?required}", false},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := missingInterpolation(test.input, lookup); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}
