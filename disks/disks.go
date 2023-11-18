package disks

import (
	"fmt"
	"io"
	"strings"

	"github.com/gocarina/gocsv"
)

/////////////////////////////////////////////////////////////////////////////////
// Formatter options

type BasicFormatterOptions interface {
	Metadata() any
	TotalSizeBytes() int64
}

type FormatterOptionsWithMaxFiles interface {
	BasicFormatterOptions
	MaxFiles() int64
}

type FormatterWithGeometryOptions struct {
	Geometry DiskGeometry
}

////////////////////////////////////////////////////////////////////////////////
// Geometry

type DiskGeometry struct {
	Name               string `csv:"name"`
	Slug               string `csv:"slug"`
	FirstYearAvailable uint   `csv:"first_year_available"`
	FormFactor         string `csv:"form_factor"`
	IsRemovable        uint   `csv:"is_removable"`

	// BitsPerAddressUnit gives the number of bits in the device's smallest
	// addressible unit of memory. For most devices it's a byte (8). For those
	// where this is not the case, their documentation usually refers to it as a
	// "word". 12 and 18 bits per word are common in older devices.
	BitsPerAddressUnit uint `csv:"bits_per_address_unit"`

	// AddressUnitsPerSector gives the number of address units in a sector, or
	// "record".
	AddressUnitsPerSector uint `csv:"address_units_per_sector"`
	SectorsPerTrack       uint `csv:"sectors_per_track"`

	// TotalDataTracks gives the number of data tracks per head.
	TotalDataTracks uint `csv:"total_data_tracks"`
	HiddenTracks    uint `csv:"hidden_tracks"`
	// Heads gives the number of heads in the device.
	Heads uint   `csv:"heads"`
	Notes string `csv:"notes"`
}

// TotalSizeBytes gives the size of the storage device, rounded up to the nearest
// byte. This gives the minimum size of the image file.
func (g *DiskGeometry) TotalSizeBytes() int64 {
	bits := int64(
		g.BitsPerAddressUnit * g.AddressUnitsPerSector * g.SectorsPerTrack *
			g.TotalDataTracks * g.Heads)
	if bits%8 == 0 {
		return bits / 8
	}
	return (bits / 8) + 1
}

////////////////////////////////////////////////////////////////////////////////

// https://en.wikipedia.org/wiki/List_of_floppy_disk_formats
// go:embed disk-geometries.csv
var diskGeometriesRawCSV string
var diskGeometries map[string]DiskGeometry

func GetPredefinedDiskGeometry(slug string) (DiskGeometry, error) {
	geometry, ok := diskGeometries[slug]
	if ok {
		return geometry, nil
	}

	err := fmt.Errorf("no predefined disk geometry exists with slug %q", slug)
	return DiskGeometry{}, err
}

func init() {
	reader := strings.NewReader(diskGeometriesRawCSV)
	err := gocsv.UnmarshalToCallback(
		reader,
		func(row DiskGeometry) error {
			_, exists := diskGeometries[row.Slug]
			if exists {
				return fmt.Errorf(
					"duplicate definition for disk %q found on row %d",
					row.Slug,
					len(diskGeometries)+1,
				)
			}
			diskGeometries[row.Slug] = row
			return nil
		},
	)
	if err != nil && err != io.EOF {
		panic(err)
	}
}

// TODO (dargueta): Implement load and search functions.
