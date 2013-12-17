package mapnik

import (
	"fmt"
	"log"
)

type TileCoord struct {
	X, Y, Zoom uint64
	Tms        bool
}

func (c TileCoord) OSMFilename() string {
	return fmt.Sprintf("%d/%d/%d.png", c.Zoom, c.X, c.Y)
}

type TileRenderResult struct {
	Coord   TileCoord
	BlobPNG []byte
}

type TileRenderRequest struct {
	Coord   TileCoord
	OutChan chan<- TileRenderResult
}

func (c *TileCoord) setTMS(tms bool) {
	if c.Tms != tms {
		c.Y = (1 << c.Zoom) - c.Y - 1
		c.Tms = tms
	}
}

func SetupRenderRoutine(stylesheet string) chan<- TileRenderRequest {
	c := make(chan TileRenderRequest)

	go func(requestChan <-chan TileRenderRequest) {
		var err error
		t := NewTileRenderer(stylesheet)
		for request := range requestChan {
			result := TileRenderResult{request.Coord, nil}
			result.BlobPNG, err = t.RenderTile(request.Coord)
			if err != nil {
				log.Println("Error while rendering", request.Coord, ":", err.Error())
				result.BlobPNG = nil
			}
			request.OutChan <- result
		}
	}(c)

	return c
}

// Renders images as Web Mercator tiles
type TileRenderer struct {
	m  *Map
	mp Projection
}

func NewTileRenderer(stylesheet string) *TileRenderer {
	t := new(TileRenderer)
	var err error
	if err != nil {
		log.Fatal(err)
	}
	t.m = NewMap(256, 256)
	t.m.Load(stylesheet)
	t.mp = t.m.Projection()

	return t
}

func (t *TileRenderer) RenderTile(c TileCoord) ([]byte, error) {
	c.setTMS(false)
	return t.RenderTileZXY(c.Zoom, c.X, c.Y)
}

// Render a tile with coordinates in Google tile format.
// Most upper left tile is always 0,0. Method is not thread-safe,
// so wrap in a channel communication or with a mutex when accessing
// the same renderer by multiple threads.
func (t *TileRenderer) RenderTileZXY(zoom, x, y uint64) ([]byte, error) {
	// Calculate pixel positions of bottom left & top right
	p0 := [2]float64{float64(x) * 256, (float64(y) + 1) * 256}
	p1 := [2]float64{(float64(x) + 1) * 256, float64(y) * 256}

	// Convert to LatLong(EPSG:4326)
	l0 := fromPixelToLL(p0, zoom)
	l1 := fromPixelToLL(p1, zoom)

	// Convert to map projection (e.g. mercartor co-ords EPSG:3857)
	c0 := t.mp.Forward(Coord{l0[0], l0[1]})
	c1 := t.mp.Forward(Coord{l1[0], l1[1]})

	// Bounding box for the Tile
	t.m.Resize(256, 256)
	t.m.ZoomToMinMax(c0.X, c0.Y, c1.X, c1.Y)
	t.m.SetBufferSize(128)

	blob := t.m.RenderToMemoryPng()
	return blob, nil
}