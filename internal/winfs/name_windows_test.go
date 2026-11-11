package winfs

import "testing"

func TestSameNameWindowsOrdinalCase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		a, b  string
		equal bool
	}{{"àbc", "Àbc", true}, {"K", "k", true}, {"K", "K", false}, {"NONTILDE", "Original Long Directory", false}} {
		got, err := (FS{}).SameName(tc.a, tc.b)
		if err != nil || got != tc.equal {
			t.Fatalf("Windows name comparison conflated different names: %q %q = %v %v", tc.a, tc.b, got, err)
		}
	}
}
