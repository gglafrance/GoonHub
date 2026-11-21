package playability

import (
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		video     string
		audio     string
		mode      Mode
		copyVideo bool
		copyAudio bool
		reason    string // substring of the reason ("" for direct)
	}{
		// Browser native
		{"mp4 h264 aac", "/v/a.mp4", "h264", "aac", ModeDirect, false, false, ""},
		{"upper case extension", "/v/a.MP4", "H264", "AAC", ModeDirect, false, false, ""},
		{"m4v", "/v/a.m4v", "h264", "aac", ModeDirect, false, false, ""},
		{"mov h264", "/v/a.mov", "h264", "aac", ModeDirect, false, false, ""},
		{"webm vp9 opus", "/v/a.webm", "vp9", "opus", ModeDirect, false, false, ""},
		{"webm vp8 vorbis", "/v/a.webm", "vp8", "vorbis", ModeDirect, false, false, ""},
		{"mp4 av1", "/v/a.mp4", "av1", "aac", ModeDirect, false, false, ""},
		{"mp4 no audio", "/v/a.mp4", "h264", "", ModeDirect, false, false, ""},
		{"mp4 mp3", "/v/a.mp4", "h264", "mp3", ModeDirect, false, false, ""},
		{"mp4 unknown codecs", "/v/a.mp4", "", "", ModeDirect, false, false, ""},
		{"webm unknown video", "/v/a.webm", "", "opus", ModeDirect, false, false, ""},

		// Container only: remux, streams copied
		{"mkv h264 aac", "/v/a.mkv", "h264", "aac", ModeRemux, true, true, "MKV container"},
		{"flv h264 aac", "/v/a.flv", "h264", "aac", ModeRemux, true, true, "FLV container"},
		{"ts h264 aac", "/v/a.ts", "h264", "aac", ModeRemux, true, true, "TS container"},
		{"avi h264 mp3", "/v/a.avi", "h264", "mp3", ModeRemux, true, true, "AVI container"},
		{"mkv vp9 opus", "/v/a.mkv", "vp9", "opus", ModeRemux, true, true, "MKV container"},
		// Video copied, audio re-encoded
		{"mp4 h264 ac3", "/v/a.mp4", "h264", "ac3", ModeRemux, true, false, "AC-3 audio"},
		{"mkv h264 dts", "/v/a.mkv", "h264", "dts", ModeRemux, true, false, "DTS audio"},
		{"mkv h264 flac", "/v/a.mkv", "h264", "flac", ModeRemux, true, false, "MKV container"},
		{"mkv h264 unknown audio", "/v/a.mkv", "h264", "", ModeRemux, true, false, "MKV container"},

		// Video re-encoded
		{"mp4 hevc aac", "/v/a.mp4", "hevc", "aac", ModeTranscode, false, true, "HEVC (H.265) video"},
		{"mkv hevc eac3", "/v/a.mkv", "hevc", "eac3", ModeTranscode, false, false, "E-AC-3 audio"},
		{"avi xvid mp3", "/v/a.avi", "mpeg4", "mp3", ModeTranscode, false, true, "MPEG-4 Part 2"},
		{"mpg mpeg2 mp2", "/v/a.mpg", "mpeg2video", "mp2", ModeTranscode, false, false, "MPEG-2 video"},
		{"mpeg mpeg1", "/v/a.mpeg", "mpeg1video", "mp2", ModeTranscode, false, false, "MPEG container"},
		{"wmv", "/v/a.wmv", "wmv3", "wmav2", ModeTranscode, false, false, "WMA audio"},
		{"flv sorenson", "/v/a.flv", "flv1", "mp3", ModeTranscode, false, true, "Sorenson Spark video"},
		{"mkv vp8 vorbis", "/v/a.mkv", "vp8", "vorbis", ModeTranscode, false, false, "MKV container"},
		{"ogv theora", "/v/a.ogv", "theora", "vorbis", ModeTranscode, false, false, "OGV container, Theora video"},
		{"mov prores pcm", "/v/a.mov", "prores", "pcm_s16le", ModeTranscode, false, false, "PCM audio"},
		{"avi unknown codecs", "/v/a.avi", "", "", ModeTranscode, false, false, "AVI container"},
		{"no extension", "/v/file", "h264", "aac", ModeRemux, true, true, "unknown container"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := Decide(c.path, c.video, c.audio)
			if plan.Mode != c.mode {
				t.Fatalf("mode = %q, want %q (plan %+v)", plan.Mode, c.mode, plan)
			}
			if c.mode == ModeDirect {
				if plan.Reason != "" {
					t.Fatalf("direct plan has a reason: %q", plan.Reason)
				}
				return
			}
			if plan.CopyVideo != c.copyVideo || plan.CopyAudio != c.copyAudio {
				t.Fatalf("copy video/audio = %v/%v, want %v/%v", plan.CopyVideo, plan.CopyAudio, c.copyVideo, c.copyAudio)
			}
			if !strings.Contains(plan.Reason, c.reason) {
				t.Fatalf("reason %q does not mention %q", plan.Reason, c.reason)
			}
		})
	}
}

func TestFullTranscode(t *testing.T) {
	plan := FullTranscode()
	if plan.Mode != ModeTranscode || plan.CopyVideo || plan.CopyAudio {
		t.Fatalf("unexpected plan %+v", plan)
	}
}
