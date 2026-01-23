package ffmpeg

import "math"

// CalculateTileDimensions computes the tile width and height for sprite sheets
// and thumbnails based on the video's native aspect ratio.
// The longest side is set to maxDimension, and the shorter side is scaled
// proportionally and rounded to the nearest even number.
func CalculateTileDimensions(videoWidth, videoHeight, maxDimension int) (tileWidth, tileHeight int) {
	if videoWidth <= 0 || videoHeight <= 0 || maxDimension <= 0 {
		return maxDimension, maxDimension
	}

	if videoWidth >= videoHeight {
		// Landscape or square
		tileWidth = maxDimension
		tileHeight = roundToEven(float64(maxDimension) * float64(videoHeight) / float64(videoWidth))
	} else {
		// Portrait
		tileHeight = maxDimension
		tileWidth = roundToEven(float64(maxDimension) * float64(videoWidth) / float64(videoHeight))
	}

	if tileWidth < 2 {
		tileWidth = 2
	}
	if tileHeight < 2 {
		tileHeight = 2
	}

	return tileWidth, tileHeight
}

func roundToEven(v float64) int {
	rounded := int(math.Round(v))
	if rounded%2 != 0 {
		rounded++
	}
	return rounded
}
