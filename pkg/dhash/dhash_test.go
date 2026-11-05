package dhash

import (
	"testing"
)

func TestComputeDHash_AllSame(t *testing.T) {
	// All pixels have the same value, so no pixel is greater than its right
	// neighbor. Every bit should be 0.
	pixels := make([]byte, 72)
	for i := range pixels {
		pixels[i] = 128
	}

	got := ComputeDHash(pixels)
	if got != 0 {
		t.Fatalf("ComputeDHash(all same) = 0x%016x, want 0x0000000000000000", got)
	}
}

func TestComputeDHash_DecreasingRow(t *testing.T) {
	// Every row has pixels strictly decreasing left-to-right, so every
	// comparison left > right is true and every bit should be set.
	pixels := make([]byte, 72)
	for row := 0; row < 8; row++ {
		for col := 0; col < 9; col++ {
			// 200, 190, 180, ... 120
			pixels[row*9+col] = byte(200 - col*10)
		}
	}

	got := ComputeDHash(pixels)
	var want uint64 = 0xFFFFFFFFFFFFFFFF
	if got != want {
		t.Fatalf("ComputeDHash(decreasing rows) = 0x%016x, want 0x%016x", got, want)
	}
}

func TestComputeDHash_IncreasingRow(t *testing.T) {
	// Every row has pixels strictly increasing left-to-right, so no pixel
	// is greater than its right neighbor. All bits should be 0.
	pixels := make([]byte, 72)
	for row := 0; row < 8; row++ {
		for col := 0; col < 9; col++ {
			// 10, 20, 30, ... 90
			pixels[row*9+col] = byte((col + 1) * 10)
		}
	}

	got := ComputeDHash(pixels)
	if got != 0 {
		t.Fatalf("ComputeDHash(increasing rows) = 0x%016x, want 0x0000000000000000", got)
	}
}

func TestComputeDHash_SingleBit(t *testing.T) {
	// All pixels equal except one specific pair where left > right.
	// Row 0, col 0 is bit 0.  Row 3, col 5 is bit 3*8+5 = 29.
	tests := []struct {
		name string
		row  int
		col  int
		bit  uint
	}{
		{"bit 0 (row=0, col=0)", 0, 0, 0},
		{"bit 29 (row=3, col=5)", 3, 5, 29},
		{"bit 63 (row=7, col=7)", 7, 7, 63},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pixels := make([]byte, 72)
			for i := range pixels {
				pixels[i] = 100
			}
			// Make left > right for the chosen pair
			pixels[tc.row*9+tc.col] = 200
			pixels[tc.row*9+tc.col+1] = 50

			got := ComputeDHash(pixels)
			want := uint64(1) << tc.bit
			if got != want {
				t.Fatalf("ComputeDHash(single bit %d) = 0x%016x, want 0x%016x", tc.bit, got, want)
			}
		})
	}
}

func TestComputeDHash_KnownPixels(t *testing.T) {
	// Construct a specific 72-byte pixel buffer and manually compute the
	// expected hash.
	//
	// Layout: 8 rows of 9 pixels each.
	// Row 0: 50 30 70 60 10 90 80 20 40
	//   comparisons (left > right?):
	//     50>30=T  30>70=F  70>60=T  60>10=T  10>90=F  90>80=T  80>20=T  20>40=F
	//   bits 0-7: 1 0 1 1 0 1 1 0 = 0x6D
	//
	// Rows 1-7: all pixels equal (100), so all bits 0.
	pixels := make([]byte, 72)
	for i := range pixels {
		pixels[i] = 100
	}

	// Row 0
	row0 := []byte{50, 30, 70, 60, 10, 90, 80, 20, 40}
	copy(pixels[0:9], row0)

	// Manually computed: bits 0,2,3,5,6 are set.
	// Binary: 0110_1101 = 0x6D
	var want uint64 = 0x6D

	got := ComputeDHash(pixels)
	if got != want {
		t.Fatalf("ComputeDHash(known pixels) = 0x%016x, want 0x%016x", got, want)
	}
}

func TestHammingDistance_Identical(t *testing.T) {
	values := []uint64{0, 1, 0xDEADBEEFCAFEBABE, 0xFFFFFFFFFFFFFFFF}
	for _, v := range values {
		got := HammingDistance(v, v)
		if got != 0 {
			t.Errorf("HammingDistance(0x%016x, 0x%016x) = %d, want 0", v, v, got)
		}
	}
}

func TestHammingDistance_AllDifferent(t *testing.T) {
	got := HammingDistance(0x0, 0xFFFFFFFFFFFFFFFF)
	if got != 64 {
		t.Fatalf("HammingDistance(0x0, 0xFFFFFFFFFFFFFFFF) = %d, want 64", got)
	}
}

func TestHammingDistance_OneBit(t *testing.T) {
	// Differ by exactly one bit at each possible position
	for i := uint(0); i < 64; i++ {
		a := uint64(0)
		b := uint64(1) << i
		got := HammingDistance(a, b)
		if got != 1 {
			t.Errorf("HammingDistance(0, 1<<%d) = %d, want 1", i, got)
		}
	}
}

func TestHammingDistance_Symmetric(t *testing.T) {
	pairs := [][2]uint64{
		{0x0, 0xFFFFFFFFFFFFFFFF},
		{0x123456789ABCDEF0, 0xFEDCBA9876543210},
		{0xAAAAAAAAAAAAAAAA, 0x5555555555555555},
		{42, 0},
	}

	for _, p := range pairs {
		ab := HammingDistance(p[0], p[1])
		ba := HammingDistance(p[1], p[0])
		if ab != ba {
			t.Errorf("HammingDistance(0x%016x, 0x%016x) = %d but reverse = %d",
				p[0], p[1], ab, ba)
		}
	}
}

func TestHammingDistance_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		a    uint64
		b    uint64
		want int
	}{
		{"both zero", 0, 0, 0},
		{"all different", 0x0, 0xFFFFFFFFFFFFFFFF, 64},
		{"one bit low", 0x0, 0x1, 1},
		{"one bit high", 0x0, 0x8000000000000000, 1},
		{"two bits", 0x0, 0x3, 2},
		{"alternating even", 0xAAAAAAAAAAAAAAAA, 0x5555555555555555, 64},
		{"half bits", 0x00000000FFFFFFFF, 0xFFFFFFFF00000000, 64},
		{"close hashes", 0xFF00FF00FF00FF00, 0xFF00FF00FF00FF01, 1},
		{"nibble difference", 0xF0, 0x0F, 8},
		{"real-world similar", 0xA3B1C2D4E5F60718, 0xA3B1C2D4E5F60719, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HammingDistance(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("HammingDistance(0x%016x, 0x%016x) = %d, want %d",
					tc.a, tc.b, got, tc.want)
			}
		})
	}
}
