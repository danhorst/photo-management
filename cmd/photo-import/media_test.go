package main

import (
	"testing"

	"github.com/dbh/photo-import/internal/index"
)

func TestResolveVolume(t *testing.T) {
	vols := []index.VolumeInfo{
		{VolumeID: "aabbccdd1122aabb", Label: "EOS_DIGITAL"},
		{VolumeID: "aabbccdd3344ccdd", Label: "SD_CARD"},
		{VolumeID: "ff00112233aabbcc", Label: "UNTITLED"},
	}

	// exact match
	id, err := resolveVolume("aabbccdd1122aabb", vols)
	if err != nil || id != "aabbccdd1122aabb" {
		t.Errorf("exact: got %q, %v; want aabbccdd1122aabb, nil", id, err)
	}

	// unambiguous prefix
	id, err = resolveVolume("ff00", vols)
	if err != nil || id != "ff00112233aabbcc" {
		t.Errorf("prefix: got %q, %v; want ff00112233aabbcc, nil", id, err)
	}

	// ambiguous prefix
	_, err = resolveVolume("aabbccdd", vols)
	if err == nil {
		t.Error("ambiguous prefix: expected error, got nil")
	}

	// label-only (matches no volume id prefix)
	_, err = resolveVolume("EOS_DIGITAL", vols)
	if err == nil {
		t.Error("label-only: expected error, got nil")
	}
}
