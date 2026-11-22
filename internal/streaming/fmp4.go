package streaming

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Minimal fragmented MP4 support for the transcode cache: the conversion output is
// ftyp + moov (no samples, "empty_moov") followed by moof + mdat fragments, one per video
// keyframe. Only the parts needed to index the fragments by time and to serve them from
// another start are read and rewritten (track ids and timescales in moov, decode times in
// moof).

// Largest non-media box accepted (ftyp, moov, moof): they are small, a bigger one means
// the stream is not what we expect.
const maxMP4HeaderBox = 16 << 20

var errMP4Format = errors.New("unexpected fragmented MP4 layout")

// mp4Track is a track of the init segment.
type mp4Track struct {
	ID        uint32
	Timescale uint32
	Video     bool
}

// mp4BoxHeader parses the header of the box at the start of b. size is the full box size
// (0: the box runs to the end of the stream). ok is false when b is too short.
func mp4BoxHeader(b []byte) (typ string, size uint64, hdrLen int, ok bool) {
	if len(b) < 8 {
		return "", 0, 0, false
	}
	size = uint64(binary.BigEndian.Uint32(b[0:4]))
	typ = string(b[4:8])
	hdrLen = 8
	if size == 1 {
		if len(b) < 16 {
			return "", 0, 0, false
		}
		size = binary.BigEndian.Uint64(b[8:16])
		hdrLen = 16
	}
	return typ, size, hdrLen, true
}

// eachMP4Box calls fn for every box of b with the offset of the box in b and its payload
// (after the header).
func eachMP4Box(b []byte, fn func(typ string, offset int, payload []byte) error) error {
	for pos := 0; pos < len(b); {
		typ, size, hdrLen, ok := mp4BoxHeader(b[pos:])
		if !ok {
			return fmt.Errorf("%w: truncated box header", errMP4Format)
		}
		if size == 0 {
			size = uint64(len(b) - pos)
		}
		if size < uint64(hdrLen) || size > uint64(len(b)-pos) {
			return fmt.Errorf("%w: bad size of box %q", errMP4Format, typ)
		}
		if err := fn(typ, pos, b[pos+hdrLen:pos+int(size)]); err != nil {
			return err
		}
		pos += int(size)
	}
	return nil
}

// parseMP4Tracks reads the tracks of a moov payload.
func parseMP4Tracks(moov []byte) ([]mp4Track, error) {
	var tracks []mp4Track
	err := eachMP4Box(moov, func(typ string, _ int, trak []byte) error {
		if typ != "trak" {
			return nil
		}
		var track mp4Track
		haveID, haveScale := false, false
		err := eachMP4Box(trak, func(typ string, _ int, payload []byte) error {
			switch typ {
			case "tkhd":
				// version 1 has 64-bit creation and modification times
				off := 12
				if len(payload) > 0 && payload[0] == 1 {
					off = 20
				}
				if len(payload) < off+4 {
					return fmt.Errorf("%w: short tkhd", errMP4Format)
				}
				track.ID = binary.BigEndian.Uint32(payload[off : off+4])
				haveID = true
			case "mdia":
				return eachMP4Box(payload, func(typ string, _ int, payload []byte) error {
					switch typ {
					case "mdhd":
						off := 12
						if len(payload) > 0 && payload[0] == 1 {
							off = 20
						}
						if len(payload) < off+4 {
							return fmt.Errorf("%w: short mdhd", errMP4Format)
						}
						track.Timescale = binary.BigEndian.Uint32(payload[off : off+4])
						haveScale = true
					case "hdlr":
						if len(payload) < 12 {
							return fmt.Errorf("%w: short hdlr", errMP4Format)
						}
						track.Video = string(payload[8:12]) == "vide"
					}
					return nil
				})
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !haveID || !haveScale || track.Timescale == 0 {
			return fmt.Errorf("%w: track without id or timescale", errMP4Format)
		}
		tracks = append(tracks, track)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, fmt.Errorf("%w: no track", errMP4Format)
	}
	return tracks, nil
}

// mp4DecodeTime is the base decode time (tfdt) of one track in a moof.
type mp4DecodeTime struct {
	TrackID uint32
	Time    uint64
	// offset in the moof box of the tfdt value and its width (4 or 8 bytes)
	offset int
	width  int
}

// parseMoof reads the decode time of each track of a whole moof box (header included).
func parseMoof(moof []byte) ([]mp4DecodeTime, error) {
	typ, _, hdrLen, ok := mp4BoxHeader(moof)
	if !ok || typ != "moof" {
		return nil, fmt.Errorf("%w: not a moof", errMP4Format)
	}
	var times []mp4DecodeTime
	err := eachMP4Box(moof[hdrLen:], func(typ string, trafOff int, traf []byte) error {
		if typ != "traf" {
			return nil
		}
		_, _, trafHdr, _ := mp4BoxHeader(moof[hdrLen+trafOff:])
		trafPayloadOff := hdrLen + trafOff + trafHdr
		dt := mp4DecodeTime{}
		haveID, haveTime := false, false
		err := eachMP4Box(traf, func(typ string, off int, payload []byte) error {
			_, _, boxHdr, _ := mp4BoxHeader(traf[off:])
			switch typ {
			case "tfhd":
				if len(payload) < 8 {
					return fmt.Errorf("%w: short tfhd", errMP4Format)
				}
				dt.TrackID = binary.BigEndian.Uint32(payload[4:8])
				haveID = true
			case "tfdt":
				if len(payload) < 8 {
					return fmt.Errorf("%w: short tfdt", errMP4Format)
				}
				dt.offset = trafPayloadOff + off + boxHdr + 4
				if payload[0] == 1 {
					if len(payload) < 12 {
						return fmt.Errorf("%w: short tfdt", errMP4Format)
					}
					dt.Time = binary.BigEndian.Uint64(payload[4:12])
					dt.width = 8
				} else {
					dt.Time = uint64(binary.BigEndian.Uint32(payload[4:8]))
					dt.width = 4
				}
				haveTime = true
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !haveID || !haveTime {
			return fmt.Errorf("%w: traf without tfhd or tfdt", errMP4Format)
		}
		times = append(times, dt)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(times) == 0 {
		return nil, fmt.Errorf("%w: moof without traf", errMP4Format)
	}
	return times, nil
}

// rewriteMoof sets, in place, the sequence number of a whole moof box and the decode time
// of each of its tracks (newTime returns the new time from the track and the old time).
func rewriteMoof(moof []byte, sequence uint32, newTime func(trackID uint32, old uint64) uint64) error {
	times, err := parseMoof(moof)
	if err != nil {
		return err
	}
	_, _, hdrLen, _ := mp4BoxHeader(moof)
	err = eachMP4Box(moof[hdrLen:], func(typ string, off int, payload []byte) error {
		if typ != "mfhd" {
			return nil
		}
		if len(payload) < 8 {
			return fmt.Errorf("%w: short mfhd", errMP4Format)
		}
		_, _, boxHdr, _ := mp4BoxHeader(moof[hdrLen+off:])
		binary.BigEndian.PutUint32(moof[hdrLen+off+boxHdr+4:], sequence)
		return nil
	})
	if err != nil {
		return err
	}
	for _, dt := range times {
		value := newTime(dt.TrackID, dt.Time)
		if dt.width == 4 {
			if value > 0xFFFFFFFF {
				return fmt.Errorf("%w: decode time overflows a 32-bit tfdt", errMP4Format)
			}
			binary.BigEndian.PutUint32(moof[dt.offset:], uint32(value))
		} else {
			binary.BigEndian.PutUint64(moof[dt.offset:], value)
		}
	}
	return nil
}

// firstSample returns where the first sample of a track starts, relative to the start of
// the moof (the base with default_base_moof), and its size. ok is false when the moof does
// not say it (other offset modes, no sample).
func firstSample(moof []byte, trackID uint32) (offset int64, size uint32, ok bool) {
	_, _, hdrLen, valid := mp4BoxHeader(moof)
	if !valid {
		return 0, 0, false
	}
	_ = eachMP4Box(moof[hdrLen:], func(typ string, _ int, traf []byte) error {
		if typ != "traf" || ok {
			return nil
		}
		var (
			thisTrack   bool
			defaultSize uint32
			trunData    []byte
			trunFlags   uint32
			baseIsMoof  bool
		)
		_ = eachMP4Box(traf, func(typ string, _ int, payload []byte) error {
			switch typ {
			case "tfhd":
				if len(payload) < 8 {
					return nil
				}
				flags := binary.BigEndian.Uint32(payload[0:4]) & 0xFFFFFF
				thisTrack = binary.BigEndian.Uint32(payload[4:8]) == trackID
				// base_data_offset present: absolute offsets, not supported
				baseIsMoof = flags&0x01 == 0
				pos := 8
				for _, f := range []uint32{0x01, 0x02, 0x08} {
					if flags&f != 0 {
						if f == 0x01 {
							pos += 8
						} else {
							pos += 4
						}
					}
				}
				if flags&0x10 != 0 && len(payload) >= pos+4 {
					defaultSize = binary.BigEndian.Uint32(payload[pos : pos+4])
				}
			case "trun":
				if trunData == nil && len(payload) >= 8 {
					trunData = payload
					trunFlags = binary.BigEndian.Uint32(payload[0:4]) & 0xFFFFFF
				}
			}
			return nil
		})
		if !thisTrack || !baseIsMoof || trunData == nil || trunFlags&0x01 == 0 {
			return nil
		}
		if binary.BigEndian.Uint32(trunData[4:8]) == 0 || len(trunData) < 12 {
			return nil
		}
		dataOffset := int32(binary.BigEndian.Uint32(trunData[8:12]))
		pos := 12
		if trunFlags&0x04 != 0 {
			pos += 4
		}
		if trunFlags&0x100 != 0 {
			pos += 4
		}
		sampleSize := defaultSize
		if trunFlags&0x200 != 0 {
			if len(trunData) < pos+4 {
				return nil
			}
			sampleSize = binary.BigEndian.Uint32(trunData[pos : pos+4])
		}
		if sampleSize == 0 || dataOffset < 0 {
			return nil
		}
		offset, size, ok = int64(dataOffset), sampleSize, true
		return nil
	})
	return offset, size, ok
}

// Bytes hashed at the start and at the end of the first video sample of a fragment to
// recognize a keyframe copied by two runs.
const sampleKeyBytes = 4096

// sampleKeyRanges are the parts of a sample of size bytes that make its key, relative to
// its start: the head and the tail.
func sampleKeyRanges(size uint32) (headEnd, tailStart int64) {
	n := int64(size)
	headEnd = min(n, sampleKeyBytes)
	tailStart = max(headEnd, n-sampleKeyBytes)
	return headEnd, tailStart
}

// sampleKey hashes the size, head and tail of a sample (FNV-1a), never 0.
func sampleKey(size uint32, head, tail []byte) uint64 {
	h := uint64(14695981039346656037)
	for _, part := range [][]byte{binary.BigEndian.AppendUint32(nil, size), head, tail} {
		for _, b := range part {
			h = (h ^ uint64(b)) * 1099511628211
		}
	}
	return h | 1
}
