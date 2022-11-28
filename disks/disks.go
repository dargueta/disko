package disks

import (
	"fmt"
	"strings"

	"github.com/gocarina/gocsv"
)

type DiskGeometry struct {
	BitsPerAddressUnit    uint `csv:"bits_per_address_unit"`
	AddressUnitsPerSector uint `csv:"address_units_per_sector"`
	SectorsPerTrack       uint `csv:"sectors_per_track"`
	TotalDataTracks       uint `csv:"total_data_tracks"`
	TotalTracks           uint `csv:"total_tracks"`
}

type PredefinedDiskProfile struct {
	Name               string `csv:"name"`
	Slug               string `csv:"slug"`
	FirstYearAvailable uint   `csv:"first_year_available"`
	IsRemovable        uint   `csv:"is_removable"`
	Notes              string `csv:"notes"`

	DiskGeometry
}

type FormatterOptions struct {
	TotalFiles int64
	Geometry   DiskGeometry
}

// go:embed disk-geometries.csv
var diskGeometriesRawCSV string
var diskGeometries map[string]PredefinedDiskProfile

func init() {
	reader := strings.NewReader(diskGeometriesRawCSV)
	err := gocsv.UnmarshalToCallback(
		reader,
		func(row PredefinedDiskProfile) error {
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
	if err != nil {
		panic(err)
	}
}

// TODO (dargueta): Implement load and search functions.
