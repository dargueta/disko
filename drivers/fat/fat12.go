package fat

type RawFAT12BootSector struct {
	RawFATBootSectorWithBPB
	DriveNumber     uint8
	NTReserved      uint8
	ExBootSignature uint8
	VolumeID        uint32
	VolumeLabel     [11]byte
	FileSystemType  [8]byte
}

type FAT12ReaderDriver struct {
	bootSector RawFAT12BootSector
}
