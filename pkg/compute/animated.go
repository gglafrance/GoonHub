package compute

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"goonhub/pkg/ffmpeg"
	"goonhub/pkg/workerproto"
)

// AnimatedThumbnails makes the marker clips then the scene preview into req.OutDir.
//
// As on the server: a marker that fails is recorded in MarkerFailures and the others go on;
// a failed preview is recorded in PreviewError (best effort; a preview with audio that fails
// is retried silent). Each clip has its own timeout (MarkerTimeoutSec). When ctx ends, the
// partial result made so far is returned with the context error, so the caller can still
// commit the finished clips.
func AnimatedThumbnails(ctx context.Context, env Env, req Request, p workerproto.AnimatedThumbnailsParams) (*workerproto.AnimatedThumbnailsResult, error) {
	res := &workerproto.AnimatedThumbnailsResult{Markers: []workerproto.MarkerClipResult{}}
	if err := p.Validate(); err != nil {
		return res, err
	}
	if len(p.Markers) > 0 || p.Preview != nil {
		if err := os.MkdirAll(req.OutDir, 0755); err != nil {
			return res, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	timeout := time.Duration(p.MarkerTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = workerproto.DefaultMarkerClipTimeoutSec * time.Second
	}
	steps := len(p.Markers)
	if p.Preview != nil {
		steps++
	}
	done := 0

	for _, m := range p.Markers {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		err := markerClip(ctx, env, req.Input, filepath.Join(req.OutDir, m.Output), m.TimestampSec, m.DurationSec, p.MarkerWidth, p.MarkerCRF, timeout)
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			res.MarkerFailures = append(res.MarkerFailures, workerproto.MarkerFailure{MarkerID: m.MarkerID, Error: err.Error()})
		} else {
			res.Markers = append(res.Markers, workerproto.MarkerClipResult{MarkerID: m.MarkerID, File: m.Output})
		}
		done++
		req.report(done * 100 / steps)
	}

	if p.Preview != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		withAudio, err := scenePreview(ctx, env, req, *p.Preview)
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			res.PreviewError = err.Error()
		} else {
			res.Preview = &workerproto.ScenePreviewResult{File: p.Preview.Output, WithAudio: withAudio}
		}
		req.report(100)
	}

	return res, nil
}

// MarkerClip makes one marker animated thumbnail at outPath (used by the server when a
// marker is created with animated thumbnails on). timeout 0 = the default clip timeout.
func MarkerClip(ctx context.Context, env Env, input, outPath string, timestampSec, durationSec, width, crf int, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = workerproto.DefaultMarkerClipTimeoutSec * time.Second
	}
	return markerClip(ctx, env, input, outPath, timestampSec, durationSec, width, crf, timeout)
}

func markerClip(ctx context.Context, env Env, input, outPath string, timestampSec, durationSec, width, crf int, timeout time.Duration) error {
	if err := checkLocalInput(input); err != nil {
		return err
	}
	clipCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	seek := strconv.Itoa(timestampSec)
	err := runFFmpeg(clipCtx, env, StepMarkerClip, durationSec, "ffmpeg animated thumbnail failed", func(acc StepAccel) []string {
		return markerClipArgs(acc, input, outPath, seek, durationSec, width, crf)
	})
	if err != nil {
		return fmt.Errorf("failed to extract animated thumbnail: %w", err)
	}
	return nil
}

// MarkerClipArgs returns the ffmpeg arguments of a marker clip with the step acceleration
// acc (the zero StepAccel gives the server's software command). Exported for benchmarks
// and hardware probes (pkg/compute/hwaccel).
func MarkerClipArgs(acc StepAccel, input, output string, timestampSec, durationSec, width, crf int) []string {
	return markerClipArgs(acc, input, output, strconv.Itoa(timestampSec), durationSec, width, crf)
}

// markerClipArgs encodes a short H.264 clip (no audio) starting at seek.
func markerClipArgs(acc StepAccel, input, output, seek string, duration, width, crf int) []string {
	args := ffmpeg.GetDefaultArgs()
	args = append(args, acc.globalArgs()...)
	args = append(args, acc.inputArgs()...)
	vf := acc.scale(fmt.Sprintf("%d:-2", width), "flags=bilinear")
	if acc.framesOnCPU() && acc.Encoder.Upload {
		vf += "," + acc.cpuTail()
	}
	args = append(args,
		"-ss", seek,
		"-i", input,
		"-t", strconv.Itoa(duration),
		"-c:v", acc.h264Encoder(),
		"-vf", vf,
	)
	if acc.framesOnCPU() && !acc.Encoder.Upload {
		args = append(args, "-pix_fmt", acc.pixFmt())
	}
	args = append(args, acc.qualityArgs(crf)...)
	return append(args,
		"-movflags", "+faststart",
		"-map_metadata", "-1",
		"-threads", "2",
		"-an",
		"-y",
		output,
	)
}

// scenePreview makes the preview, with audio when asked and possible: an odd audio track
// (e.g. one that ends before a seek point) must not cost the scene its preview, so a failed
// run with audio is retried silent. Returns whether the preview has audio.
func scenePreview(ctx context.Context, env Env, req Request, spec workerproto.ScenePreviewSpec) (bool, error) {
	if err := checkLocalInput(req.Input); err != nil {
		return false, err
	}
	duration := req.Video.DurationSec
	if duration <= 0 {
		return false, fmt.Errorf("scene has no duration")
	}
	out := filepath.Join(req.OutDir, spec.Output)

	run := func(hasAudio bool) error {
		prefix := "ffmpeg scene preview failed"
		if isShortScenePreview(duration, spec.Segments, spec.SegmentDuration) {
			prefix = "ffmpeg scene preview (short mode) failed"
		}
		return runFFmpeg(ctx, env, StepScenePreview, duration, prefix, func(acc StepAccel) []string {
			return scenePreviewArgs(acc, req.Input, out, duration, spec.Segments, spec.SegmentDuration, spec.Width, spec.CRF, hasAudio)
		})
	}

	err := run(spec.WithAudio)
	if err == nil {
		return spec.WithAudio, nil
	}
	if spec.WithAudio && ctx.Err() == nil {
		if err2 := run(false); err2 == nil {
			return false, nil
		} else {
			err = err2
		}
	}
	return false, fmt.Errorf("failed to generate scene preview: %w", err)
}

// isShortScenePreview reports whether the video is too short to sample and is encoded whole.
func isShortScenePreview(duration int, segments int, segmentDuration float64) bool {
	return float64(duration) < float64(segments)*segmentDuration
}

// ScenePreviewArgs returns the ffmpeg arguments of a scene preview with the step
// acceleration acc (the zero StepAccel gives the server's software command). Exported for
// benchmarks and hardware probes (pkg/compute/hwaccel).
func ScenePreviewArgs(acc StepAccel, input, output string, durationSec, segments int, segmentDuration float64, width, crf int, hasAudio bool) []string {
	return scenePreviewArgs(acc, input, output, durationSec, segments, segmentDuration, width, crf, hasAudio)
}

// scenePreviewArgs samples segments evenly through the video and concatenates them; a
// video shorter than segments*segmentDuration is encoded whole at reduced resolution. With
// hasAudio, each segment keeps its audio (AAC stereo, 50 ms fades at every cut).
func scenePreviewArgs(acc StepAccel, input, output string,
	duration int, segments int, segmentDuration float64, width, crf int, hasAudio bool) []string {

	audioArgs := []string{"-c:a", "aac", "-b:a", "64k", "-ac", "2"}
	scale := acc.scale(fmt.Sprintf("%d:-2", width), "flags=bilinear")

	if isShortScenePreview(duration, segments, segmentDuration) {
		args := ffmpeg.GetDefaultArgs()
		args = append(args, acc.globalArgs()...)
		args = append(args, acc.inputArgs()...)
		args = append(args, "-i", input)
		if hasAudio {
			args = append(args, "-map", "0:v:0", "-map", "0:a:0")
		}
		vf := scale
		if acc.framesOnCPU() && acc.Encoder.Upload {
			vf += "," + acc.cpuTail()
		}
		args = append(args,
			"-c:v", acc.h264Encoder(),
			"-vf", vf,
		)
		if acc.framesOnCPU() && !acc.Encoder.Upload {
			args = append(args, "-pix_fmt", acc.pixFmt())
		}
		args = append(args, acc.qualityArgs(crf)...)
		args = append(args,
			"-movflags", "+faststart",
			"-map_metadata", "-1",
			"-threads", "4",
		)
		if hasAudio {
			args = append(args, audioArgs...)
		} else {
			args = append(args, "-an")
		}
		return append(args, "-y", output)
	}

	interval := float64(duration) / float64(segments)
	args := ffmpeg.GetDefaultArgs()
	args = append(args, acc.globalArgs()...)
	for i := 0; i < segments; i++ {
		seekPos := interval*float64(i) + interval/2
		args = append(args, acc.inputArgs()...)
		args = append(args, "-ss", fmt.Sprintf("%.2f", seekPos), "-i", input)
	}

	videoTail := scale
	if acc.framesOnCPU() {
		videoTail += "," + acc.cpuTail()
	}
	var filterParts []string
	var concatInputs []string
	for i := 0; i < segments; i++ {
		label := fmt.Sprintf("v%d", i)
		filterParts = append(filterParts,
			fmt.Sprintf("[%d:v]trim=0:%.2f,setpts=PTS-STARTPTS,%s[%s]", i, segmentDuration, videoTail, label))
		concatInputs = append(concatInputs, fmt.Sprintf("[%s]", label))
		if hasAudio {
			// Same format for every piece so concat accepts them; 50ms fades avoid a click at each cut
			audioLabel := fmt.Sprintf("a%d", i)
			filterParts = append(filterParts,
				fmt.Sprintf("[%d:a:0]atrim=0:%.2f,asetpts=PTS-STARTPTS,aformat=sample_rates=44100:channel_layouts=stereo,"+
					"afade=t=in:d=0.05,afade=t=out:st=%.2f:d=0.05[%s]",
					i, segmentDuration, segmentDuration-0.05, audioLabel))
			concatInputs = append(concatInputs, fmt.Sprintf("[%s]", audioLabel))
		}
	}
	if hasAudio {
		filterParts = append(filterParts,
			fmt.Sprintf("%sconcat=n=%d:v=1:a=1[out][aout]", strings.Join(concatInputs, ""), segments))
	} else {
		filterParts = append(filterParts,
			fmt.Sprintf("%sconcat=n=%d:v=1:a=0[out]", strings.Join(concatInputs, ""), segments))
	}

	args = append(args,
		"-filter_complex", strings.Join(filterParts, ";"),
		"-map", "[out]",
	)
	if hasAudio {
		args = append(args, "-map", "[aout]")
	}
	args = append(args, "-c:v", acc.h264Encoder())
	args = append(args, acc.qualityArgs(crf)...)
	args = append(args,
		"-movflags", "+faststart",
		"-map_metadata", "-1",
		"-threads", "4",
	)
	if hasAudio {
		args = append(args, audioArgs...)
	} else {
		args = append(args, "-an")
	}
	return append(args, "-y", output)
}
