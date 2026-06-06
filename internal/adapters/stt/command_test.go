package stt

import (
	"reflect"
	"testing"
)

func TestBuildArgsAppendsAudioPathWhenNoPlaceholder(t *testing.T) {
	got := buildArgs([]string{"--language", "es"}, "voice.oga")
	want := []string{"--language", "es", "voice.oga"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestBuildArgsReplacesAudioPlaceholder(t *testing.T) {
	got := buildArgs([]string{"--file={audio}", "--language", "es"}, "voice.oga")
	want := []string{"--file=voice.oga", "--language", "es"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
