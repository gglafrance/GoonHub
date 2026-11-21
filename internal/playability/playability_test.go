package playability

import (
	"reflect"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		video   string
		audio   string
		want    Status
		reasons []string
	}{
		{"mp4 h264 aac", "/m/a.mp4", "h264", "aac", StatusPlayable, nil},
		{"upper-case extension and codec", "/m/a.MP4", "H264", "AAC", StatusPlayable, nil},
		{"webm vp9 opus", "/m/a.webm", "vp9", "opus", StatusPlayable, nil},
		{"no audio track", "/m/a.m4v", "av1", "", StatusPlayable, nil},
		{"mov h264", "/m/a.mov", "h264", "mp3", StatusPlayable, nil},
		{"metadata not extracted", "/m/a.avi", "", "", StatusUnknown, nil},
		{"avi container", "/m/a.avi", "h264", "mp3", StatusUnplayable, []string{ReasonContainer}},
		{"mkv hevc", "/m/a.mkv", "hevc", "aac", StatusUnplayable, []string{ReasonContainer, ReasonVideoCodec}},
		{"mpeg-2 in mp4", "/m/a.mp4", "mpeg2video", "mp2", StatusUnplayable, []string{ReasonVideoCodec, ReasonAudioCodec}},
		{"hevc in mp4", "/m/a.mp4", "hevc", "aac", StatusUnplayable, []string{ReasonVideoCodec}},
		{"ac3 audio", "/m/a.mp4", "h264", "ac3", StatusUnplayable, []string{ReasonAudioCodec}},
		{"no extension", "/m/file", "h264", "aac", StatusUnplayable, []string{ReasonContainer}},
		{"dot in directory only", "/m.mp4/file", "h264", "aac", StatusUnplayable, []string{ReasonContainer}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Check(c.path, c.video, c.audio)
			if got.Status != c.want || !reflect.DeepEqual(got.Reasons, c.reasons) {
				t.Fatalf("Check(%q, %q, %q) = %+v, want %s %v", c.path, c.video, c.audio, got, c.want, c.reasons)
			}
			if IsBrowserPlayable(c.path, c.video, c.audio) != (c.want == StatusPlayable) {
				t.Fatalf("IsBrowserPlayable disagrees with Check for %q", c.name)
			}
		})
	}
}

func TestSQLUnplayable(t *testing.T) {
	cond, args := SQLUnplayable("s.stored_path", "s.video_codec", "s.audio_codec")
	if strings.Count(cond, "?") != len(args) {
		t.Fatalf("placeholders (%d) and args (%d) differ: %s", strings.Count(cond, "?"), len(args), cond)
	}
	for _, col := range []string{"s.stored_path", "s.video_codec", "s.audio_codec"} {
		if !strings.Contains(cond, col) {
			t.Fatalf("condition does not use column %s: %s", col, cond)
		}
	}
}
