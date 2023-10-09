package unixv1

import (
	"bytes"
	"encoding/binary"
	"os"
	"time"

	"github.com/dargueta/disko"
)

const NumInodesPerBlock = 16

type RawInode struct {
	Flags            uint16
	Nlinks           uint8
	UserID           uint8
	Size             uint16
	Blocks           [8]PhysicalBlock
	CreatedTime      uint32
	LastModifiedTime uint32
	Unused           uint16
}

type Inode struct {
	disko.FileStat
	rawFlags uint16
	blocks   []PhysicalBlock
}

func (inode *Inode) IsAllocated() bool {
	return inode.rawFlags&FlagFileAllocated != 0
}

func (inode *Inode) GetInodeType() os.FileMode {
	return inode.ModeFlags & disko.S_IFMT
}

func RawInodeToInode(inumber Inumber, raw RawInode) Inode {
	sizeInBlocks := (raw.Size + (-raw.Size % 512)) / 512
	return Inode{
		blocks:   raw.Blocks[:],
		rawFlags: raw.Flags,
		FileStat: disko.FileStat{
			InodeNumber:  uint64(inumber),
			Nlinks:       uint64(raw.Nlinks),
			ModeFlags:    ConvertFSFlagsToStandard(raw.Flags),
			Uid:          uint32(raw.UserID),
			BlockSize:    512,
			NumBlocks:    int64(sizeInBlocks),
			Size:         int64(raw.Size),
			CreatedAt:    fsEpoch.Add(time.Second * time.Duration(raw.CreatedTime)),
			LastModified: fsEpoch.Add(time.Second * time.Duration(raw.LastModifiedTime)),
		},
	}
}

func BytesToInode(inumber Inumber, data []byte) Inode {
	var raw RawInode

	reader := bytes.NewReader(data)
	binary.Read(reader, binary.LittleEndian, &raw)
	return RawInodeToInode(inumber, raw)
}

func InodeToRawInode(inode Inode) (Inumber, RawInode) {
	raw := RawInode{
		Flags:  ConvertStandardFlagsToFS(inode.ModeFlags),
		Nlinks: uint8(inode.Nlinks),
		UserID: uint8(inode.Uid),
		Size:   uint16(inode.Size),
	}
	copy(raw.Blocks[:], inode.blocks)
	return Inumber(inode.InodeNumber), raw
}
