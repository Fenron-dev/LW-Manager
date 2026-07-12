package thumbnail

import "testing"

func TestFitPreservesBounds(t *testing.T) {
	w, h := fit(4000, 2000, 900, 600)
	if w != 900 || h != 450 {
		t.Fatalf("fit = %dx%d", w, h)
	}
	w, h = fit(320, 200, 900, 600)
	if w != 320 || h != 200 {
		t.Fatalf("small fit = %dx%d", w, h)
	}
}
