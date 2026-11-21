package playability

import "strings"

// Conversion side of the rule: how a file Check does not accept reaches the browser
// anyway, through the on-the-fly conversion to a fragmented MP4 (see streaming.Transcoder).

// Mode tells how a scene reaches the browser.
type Mode string

const (
	// ModeDirect streams the original file as is.
	ModeDirect Mode = "direct"
	// ModeRemux copies the video stream into a fragmented MP4 (only the container, and
	// possibly the audio, are converted). Cheap.
	ModeRemux Mode = "remux"
	// ModeTranscode re-encodes the video to H.264. CPU heavy.
	ModeTranscode Mode = "transcode"
)

// Plan is the conversion decision for one file.
type Plan struct {
	Mode Mode
	// CopyVideo / CopyAudio: the stream goes into the MP4 untouched (otherwise it is
	// re-encoded to H.264 / AAC). Meaningless for ModeDirect.
	CopyVideo bool
	CopyAudio bool
	// AudioCodec is the normalized codec of the copied audio ("" when the audio is
	// re-encoded). AAC from MPEG-TS/HLS sources is in ADTS form, which the MP4 muxer only
	// takes through the aac_adtstoasc bitstream filter.
	AudioCodec string
	// Human readable list of what browsers can't play ("" for ModeDirect).
	Reason string
}

// Playable codecs that can be copied as is into a fragmented MP4 (VP8 and Vorbis/FLAC
// can't, they are re-encoded).
var mp4CopyVideoCodecs = []string{"h264", "vp9", "av1"}
var mp4CopyAudioCodecs = []string{"aac", "mp3", "opus"}

var codecLabels = map[string]string{
	"hevc":       "HEVC (H.265)",
	"mpeg2video": "MPEG-2",
	"mpeg1video": "MPEG-1",
	"mpeg4":      "MPEG-4 Part 2 (DivX/Xvid)",
	"msmpeg4v1":  "MS MPEG-4",
	"msmpeg4v2":  "MS MPEG-4",
	"msmpeg4v3":  "MS MPEG-4",
	"wmv1":       "WMV",
	"wmv2":       "WMV",
	"wmv3":       "WMV",
	"vc1":        "VC-1",
	"flv1":       "Sorenson Spark",
	"h263":       "H.263",
	"theora":     "Theora",
	"prores":     "ProRes",
	"mjpeg":      "Motion JPEG",
	"ac3":        "AC-3",
	"eac3":       "E-AC-3",
	"dts":        "DTS",
	"truehd":     "TrueHD",
	"mp2":        "MP2",
	"wmav1":      "WMA",
	"wmav2":      "WMA",
	"wmapro":     "WMA Pro",
}

func codecLabel(codec string) string {
	if label, ok := codecLabels[codec]; ok {
		return label
	}
	if strings.HasPrefix(codec, "pcm_") {
		return "PCM"
	}
	return strings.ToUpper(codec)
}

// Decide applies Check and, for a file browsers can't play, chooses between a remux (the
// video codec is fine, only the container and/or the audio are converted) and a full
// transcode. A file whose codecs are not known yet (StatusUnknown) is tried directly in a
// browser container and transcoded otherwise.
func Decide(path, videoCodec, audioCodec string) Plan {
	container := Container(path)
	video := normalizeCodec(videoCodec)
	audio := normalizeCodec(audioCodec)

	var problems []string
	switch result := Check(path, videoCodec, audioCodec); result.Status {
	case StatusPlayable:
		return Plan{Mode: ModeDirect}
	case StatusUnknown:
		if contains(playableContainers, container) {
			return Plan{Mode: ModeDirect}
		}
		problems = append(problems, containerLabel(container))
	default:
		for _, reason := range result.Reasons {
			switch reason {
			case ReasonContainer:
				problems = append(problems, containerLabel(container))
			case ReasonVideoCodec:
				problems = append(problems, codecLabel(video)+" video")
			case ReasonAudioCodec:
				problems = append(problems, codecLabel(audio)+" audio")
			}
		}
	}

	plan := Plan{
		CopyVideo: contains(mp4CopyVideoCodecs, video),
		CopyAudio: contains(mp4CopyAudioCodecs, audio),
		Reason:    "Not supported by browsers: " + strings.Join(problems, ", "),
	}
	if plan.CopyAudio {
		plan.AudioCodec = audio
	}
	if plan.CopyVideo {
		plan.Mode = ModeRemux
	} else {
		plan.Mode = ModeTranscode
	}
	return plan
}

func containerLabel(container string) string {
	if container == "" {
		return "unknown container"
	}
	return strings.ToUpper(container) + " container"
}

// FullTranscode re-encodes both streams. Used when the browser failed on a file Check
// accepts (e.g. 10-bit or 4:4:4 H.264, which the stored codec name can't tell).
func FullTranscode() Plan {
	return Plan{Mode: ModeTranscode, Reason: "Re-encoded at the player's request"}
}
