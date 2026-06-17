package main

import "testing"

func TestResolveSpriteMapsUnityBuiltinSquare(t *testing.T) {
	got := resolveSprite(nil, unityBuiltinSpriteGUID, unitySquareSpriteID)
	if got != unitySquareSpriteName {
		t.Fatalf("resolveSprite builtin square = %q, want %q", got, unitySquareSpriteName)
	}
}

func TestResolveSpriteUsesSpriteTableForImportedSprites(t *testing.T) {
	tables := map[string]*spriteTable{
		"guid": {byID: map[int64]string{42: "face_0"}},
	}
	got := resolveSprite(tables, "guid", 42)
	if got != "face_0" {
		t.Fatalf("resolveSprite imported sprite = %q, want face_0", got)
	}
}
