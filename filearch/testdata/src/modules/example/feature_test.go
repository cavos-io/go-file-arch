package example

import "testing"

func TestNewFeature(t *testing.T) {
	if NewFeature() == nil {
		t.Fatal("NewFeature() = nil")
	}
}
