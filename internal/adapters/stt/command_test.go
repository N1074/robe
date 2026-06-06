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

func TestCleanTranscriptRemovesWhisperLogs(t *testing.T) {
	output := `read_audio_data: reading audio data from '/tmp/tmp.wav' ...
read_audio_data: trying to decode with miniaudio

 Puedes preparar una cita de calendario mañana a las 11 para ver al dentista.

whisper_print_timings: total time = 10523.16 ms`

	got := cleanTranscript(output)
	want := "Puedes preparar una cita de calendario mañana a las 11 para ver al dentista."

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestCleanTranscriptJoinsTranscriptLines(t *testing.T) {
	output := "Primera frase.\nSegunda frase."
	got := cleanTranscript(output)
	want := "Primera frase. Segunda frase."

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
