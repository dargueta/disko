package fat

type RawFAT32BootSector struct {
	RawFATBootSectorWithBPB
	fatSize32        uint32
	ExtFlags         uint16
	FSVersionMinor   uint8
	FSVersionMajor   uint8
	RootCluster      uint32
	BackupBootSector uint32
	reserved         [12]byte
	DriveNumber      uint8
	NTReserved       uint8
	ExBootSignature  uint8
	VolumeID         uint32
	VolumeLabel      [11]byte
	FileSystemType   [8]byte
}
