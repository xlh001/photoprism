package thumb

import (
	"errors"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/photoprism/photoprism/pkg/fs"
)

func TestVipsCheckPixels(t *testing.T) {
	VipsInit()

	max := fs.MaxImagePixels
	defer func() { fs.MaxImagePixels = max }()

	t.Run("WithinBudget", func(t *testing.T) {
		fs.MaxImagePixels = max
		img, err := vips.LoadImageFromFile("testdata/example.jpg", VipsImportParams())
		assert.NoError(t, err)
		defer img.Close()
		assert.NoError(t, vipsCheckPixels(img, "example.jpg"))
	})
	t.Run("AboveBudget", func(t *testing.T) {
		fs.MaxImagePixels = 4
		img, err := vips.LoadImageFromFile("testdata/example.jpg", VipsImportParams())
		assert.NoError(t, err)
		defer img.Close()
		assert.True(t, errors.Is(vipsCheckPixels(img, "example.jpg"), fs.ErrImageTooLarge))
	})
	t.Run("Disabled", func(t *testing.T) {
		fs.MaxImagePixels = 0
		img, err := vips.LoadImageFromFile("testdata/example.jpg", VipsImportParams())
		assert.NoError(t, err)
		defer img.Close()
		assert.NoError(t, vipsCheckPixels(img, "example.jpg"))
	})
	t.Run("NotLoaded", func(t *testing.T) {
		fs.MaxImagePixels = max
		assert.Error(t, vipsCheckPixels(nil, "example.jpg"))
	})
}

func TestVips_PixelBudget(t *testing.T) {
	max := fs.MaxImagePixels
	defer func() { fs.MaxImagePixels = max }()

	t.Run("AboveBudget", func(t *testing.T) {
		fs.MaxImagePixels = 4
		name, buf, err := Vips("testdata/example.jpg", nil, "193456789012345678901234567890abcdef1234", t.TempDir(), 150, 150, ResampleFit)
		assert.True(t, errors.Is(err, fs.ErrImageTooLarge))
		assert.Empty(t, name)
		assert.Empty(t, buf)
	})
	t.Run("WithinBudget", func(t *testing.T) {
		fs.MaxImagePixels = max
		_, buf, err := Vips("testdata/example.jpg", nil, "193456789012345678901234567890abcdef1234", t.TempDir(), 150, 150, ResampleFit)
		assert.NoError(t, err)
		assert.NotEmpty(t, buf)
	})
}

func TestVerify_PixelBudget(t *testing.T) {
	max := fs.MaxImagePixels
	lib := Library
	defer func() {
		fs.MaxImagePixels = max
		Library = lib
	}()

	Library = LibVips

	t.Run("AboveBudget", func(t *testing.T) {
		fs.MaxImagePixels = 4
		assert.True(t, errors.Is(Verify("testdata/example.jpg"), fs.ErrImageTooLarge))
	})
	t.Run("WithinBudget", func(t *testing.T) {
		fs.MaxImagePixels = max
		assert.NoError(t, Verify("testdata/example.jpg"))
	})
}

func TestVipsLoadedPages(t *testing.T) {
	VipsInit()

	t.Run("SinglePage", func(t *testing.T) {
		img, err := vips.LoadImageFromFile("testdata/example.jpg", VipsImportParams())
		assert.NoError(t, err)
		defer img.Close()
		assert.Equal(t, 1, vipsLoadedPages(img))
	})
	t.Run("AnimatedCountsWhatWasLoaded", func(t *testing.T) {
		// The file declares many pages, but the import parameters ask for one, so counting the
		// declared pages would reject a file whose frames are never decoded.
		name := writeAnimatedGif(t, 60)
		img, err := vips.LoadImageFromFile(name, VipsImportParams())
		assert.NoError(t, err)
		defer img.Close()
		assert.Greater(t, img.Pages(), 1, "the fixture must declare more than one page")
		assert.Equal(t, 1, vipsLoadedPages(img))
	})
}

func TestVipsCheckPixels_Animated(t *testing.T) {
	VipsInit()

	max := fs.MaxImagePixels
	defer func() { fs.MaxImagePixels = max }()

	name := writeAnimatedGif(t, 60)

	t.Run("DeclaredFramesDoNotCount", func(t *testing.T) {
		// A budget that admits one frame but not sixty must admit this file, since only the
		// first frame is loaded.
		fs.MaxImagePixels = animatedGifWidth * animatedGifHeight * 2
		img, err := vips.LoadImageFromFile(name, VipsImportParams())
		assert.NoError(t, err)
		defer img.Close()
		assert.NoError(t, vipsCheckPixels(img, "animated.gif"))
	})
	t.Run("LoadedFrameStillCounts", func(t *testing.T) {
		fs.MaxImagePixels = 4
		img, err := vips.LoadImageFromFile(name, VipsImportParams())
		assert.NoError(t, err)
		defer img.Close()
		assert.ErrorIs(t, vipsCheckPixels(img, "animated.gif"), fs.ErrImageTooLarge)
	})
	t.Run("ThumbnailSucceeds", func(t *testing.T) {
		fs.MaxImagePixels = animatedGifWidth * animatedGifHeight * 2
		_, buf, err := Vips(name, nil, "293456789012345678901234567890abcdef1234", t.TempDir(), 64, 64, ResampleFit)
		assert.NoError(t, err)
		assert.NotEmpty(t, buf)
	})
}

const animatedGifWidth = 200
const animatedGifHeight = 100

// writeAnimatedGif writes an animated GIF with the given number of frames and returns its path.
func writeAnimatedGif(t *testing.T, frames int) string {
	t.Helper()

	g := &gif.GIF{}

	for i := 0; i < frames; i++ {
		p := image.NewPaletted(image.Rect(0, 0, animatedGifWidth, animatedGifHeight), color.Palette{color.Black, color.White})
		p.Set(i%animatedGifWidth, 0, color.White)
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 5)
	}

	name := filepath.Join(t.TempDir(), "animated.gif")

	// #nosec G304 -- the path is a test-owned temporary directory.
	f, err := os.Create(name)
	require.NoError(t, err)
	require.NoError(t, gif.EncodeAll(f, g))
	require.NoError(t, f.Close())

	return name
}
