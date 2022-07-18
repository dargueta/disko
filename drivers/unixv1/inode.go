package unixv1

import (
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/drivers/common"
)

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
	IsAllocated bool
	blocks      []PhysicalBlock
}

func (inode *Inode) GetInodeType() int {
	return int(inode.ModeFlags & disko.S_IFMT)
}

func (inode *Inode) IsDir() bool {
	return inode.GetInodeType() == disko.S_IFDIR
}

func (inode *Inode) IsFile() bool {
	return inode.GetInodeType() == disko.S_IFREG
}

func RawInodeToInode(inumber Inumber, raw RawInode) Inode {
	sizeInBlocks := (raw.Size + (-raw.Size % 512)) / 512
	return Inode{
		IsAllocated: raw.Flags&FlagFileAllocated != 0,
		blocks:      raw.Blocks[:],
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

type InodeManager struct {
	allocator   common.Allocator
	blockStream common.BlockStream
}

func InodeManagerFromBitmap(blockStream common.BlockStream, allocationMap []byte) InodeManager {
	return InodeManager{
		blockStream: blockStream,
		allocator:   common.NewAllocatorFromInUseBitmap(allocationMap),
	}
}
