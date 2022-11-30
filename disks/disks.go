package disks

import (
	"fmt"
	"strings"

	"github.com/gocarina/gocsv"
)

type DiskGeometry struct {
	Name                  string `csv:"name"`
	slug                  string `csv:"slug"`
	FirstYearAvailable    uint   `csv:"first_year_available"`
	IsRemovable           uint   `csv:"is_removable"`
	BitsPerAddressUnit    uint   `csv:"bits_per_address_unit"`
	AddressUnitsPerSector uint   `csv:"address_units_per_sector"`
	SectorsPerTrack       uint   `csv:"sectors_per_track"`
	TotalDataTracks       uint   `csv:"total_data_tracks"`
	TotalTracks           uint   `csv:"total_tracks"`
	notes                 string `csv:"notes"`
}

type FormatterOptions struct {
	TotalFiles int64
	Geometry   DiskGeometry
}

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
			_, exists := diskGeometries[row.slug]
			if exists {
				return fmt.Errorf(
					"duplicate definition for disk %q found on row %d",
					row.slug,
					len(diskGeometries)+1,
				)
			}
			diskGeometries[row.slug] = row
			return nil
		},
	)
	if err != nil {
		panic(err)
	}
}

// TODO (dargueta): Implement load and search functions.
