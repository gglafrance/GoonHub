// Package hwaccel finds and chooses the hardware acceleration of goonhub-worker for the
// steps of pkg/compute (sprite frames, marker clips, scene preview, visual hash).
//
//	report, err := hwaccel.Detect(ctx, ffmpegPath)      // what really works here
//	accel := hwaccel.Choose(report, hwaccel.Prefs{})    // compute.Accel per step
//	env := compute.Env{FFmpegPath: ffmpegPath, Accel: accel}
//	bench, err := hwaccel.Bench(ctx, ffmpegPath, accel, hwaccel.Sample{}) // optional
//
// Detect parses "ffmpeg -hwaccels / -encoders / -filters", then proves every candidate with
// real runs on a generated 4 s clip: a strict decode whose frames must come out of the GPU
// (hwdownload fails when ffmpeg silently fell back to software), an encode with the real
// quality options, the GPU scaler, and each step pipeline run end to end through pkg/compute
// without software fallback, its output checked (dimensions, duration, decodes; hashes
// against software). Only proven pipelines are chosen.
//
// Methods (AllMethods, preference order): NVIDIA (NVDEC + scale_npp / scale_cuda + NVENC,
// Windows / Linux), VideoToolbox (macOS), Intel Quick Sync, AMD AMF (encoder only), VA-API
// (Linux, /dev/dri/renderD128), D3D11VA (Windows, decode only).
//
// Per step (balanced preference):
//
//   - marker clips: GPU decode, the software scaler (same pixels as the server), GPU H.264
//     encoder with the CRF mapped to its own scale (see Encoder). With PreferSpeed the GPU
//     also scales.
//   - scene preview: software decode (one input per segment: as many GPU decoders at once
//     stall ffmpeg, see encodePipeline), software scaler, GPU encoder.
//   - visual hash: GPU decode, frames kept in GPU memory through fps=1/2, converted to nv12,
//     downloaded and scaled to 9x8 on the CPU. A GPU scaler never makes the 9x8 thumbnail:
//     scale_cuda straight to 9x8 moves the dHashes by 12-25 bits (it samples a few pixels
//     where swscale averages the area), which would break matching against the indexed
//     hashes. Hardware decoders always deblock, so their frames equal a full software
//     decode: the hashes differ from the indexed ones (skip_loop_filter) exactly like a
//     software full decode does, well within the 5-bit match tolerance.
//   - sprite frames: software unless Prefs.SpriteFrames or a Bench says otherwise (one short
//     ffmpeg run per frame: the GPU initialisation costs more than the decode); when on, at
//     most SpriteMaxParallel runs at once.
//
// An accelerated step that fails at run time is retried in software by pkg/compute.
//
// VideoToolbox, VA-API and Quick Sync could not be run on the development machine (Windows,
// RTX 2080 Ti + AMD iGPU): their pipelines follow the ffmpeg documentation, their arguments
// are unit tested, and Detect proves or rejects them on the real machines.
package hwaccel
