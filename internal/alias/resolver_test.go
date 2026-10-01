package alias

import (
	"jvm/internal/sources"
	"testing"
)

func TestAliases(t *testing.T) {
	releases := []sources.JavaRelease{
		{Version: "21.0.9+1", FullVersion: "21.0.9+1", MajorVersion: 21, LTS: true, Source: "adoptium"},
		{Version: "21.0.10+2", FullVersion: "21.0.10+2", MajorVersion: 21, LTS: true, Source: "adoptium"},
		{Version: "17.0.18+1", FullVersion: "17.0.18+1", MajorVersion: 17, LTS: true, Source: "adoptium"},
	}
	r := NewResolver()
	for input, want := range map[string]string{"lts": "21.0.10+2", "lts-1": "17.0.18+1", "21": "21.0.10+2", "21.0.10": "21.0.10+2"} {
		got, err := r.ResolveAlias(input, releases, "adoptium")
		if err != nil || got.Version != want {
			t.Fatalf("%s: %v %v", input, got, err)
		}
	}
	if _, err := r.ResolveAlias("21", releases, "zulu"); err == nil {
		t.Fatal("source constraint ignored")
	}
	if _, err := r.ResolveAlias("lts--1", releases, ""); err == nil {
		t.Fatal("negative offset accepted")
	}
}
