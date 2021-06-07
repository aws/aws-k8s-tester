package ecr

import "testing"

func TestIsEmpty(t *testing.T) {
	repo := &Repository{}
	repo = nil
	if !repo.IsEmpty() {
		t.Fatal("unexpected repo.IsEmpty")
	}
}
