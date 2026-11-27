package hwaccel

import "goonhub/pkg/dhash"

// MatchHammingMax is the server's default visual match tolerance
// (duplication.visual_hamming_max): two frame hashes match within 5 bits. The index lookup
// also needs one of the four 16-bit chunks to be equal, which a distance of 3 bits or less
// guarantees.
const MatchHammingMax = 5

// HashDrift compares visual hashes with reference ones, frame by frame.
type HashDrift struct {
	Frames          int     `json:"frames"`
	ReferenceFrames int     `json:"reference_frames"`
	Compared        int     `json:"compared"`
	Exact           int     `json:"exact"`    // distance 0
	Within3         int     `json:"within_3"` // distance <= 3: found by the chunk lookup for sure
	Within5         int     `json:"within_5"` // distance <= MatchHammingMax
	Mean            float64 `json:"mean"`
	Max             int     `json:"max"`
}

// CompareHashes compares got with ref frame by frame (over the shorter list).
func CompareHashes(ref, got []uint64) HashDrift {
	d := HashDrift{Frames: len(got), ReferenceFrames: len(ref)}
	n := len(ref)
	if len(got) < n {
		n = len(got)
	}
	sum := 0
	for i := 0; i < n; i++ {
		h := dhash.HammingDistance(ref[i], got[i])
		sum += h
		if h == 0 {
			d.Exact++
		}
		if h <= 3 {
			d.Within3++
		}
		if h <= MatchHammingMax {
			d.Within5++
		}
		if h > d.Max {
			d.Max = h
		}
	}
	d.Compared = n
	if n > 0 {
		d.Mean = float64(sum) / float64(n)
	}
	return d
}

// Within5Ratio is the share of compared frames within MatchHammingMax.
func (d HashDrift) Within5Ratio() float64 {
	if d.Compared == 0 {
		return 0
	}
	return float64(d.Within5) / float64(d.Compared)
}
