package app

import "testing"

func TestNewAppRegistersSafetyModules(t *testing.T) {
	app := New()

	got := app.ModuleNames()
	want := []string{"registry", "limits", "pauser"}

	if len(got) != len(want) {
		t.Fatalf("expected %d modules, got %d: %v", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected module %q at index %d, got %q", want[i], i, got[i])
		}
	}
}
