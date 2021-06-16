package common

import (
	"fmt"
)

type ClusterID uint

type ClusterStream struct {
	BlockStream       *BlockStream
	BlocksPerCluster  uint
	FirstBlock        BlockID
	FirstValidCluster ClusterID
	LastValidCluster  ClusterID
	bytesPerCluster   uint
}

// TODO (dargueta): Parameter validation
func NewClusterStream(
	blockStream *BlockStream,
	blocksPerCluster uint,
	firstBlock BlockID,
	firstValidCluster ClusterID,
	lastValidCluster ClusterID,
) (ClusterStream, error) {
	return ClusterStream{
		BlockStream:       blockStream,
		BlocksPerCluster:  blocksPerCluster,
		FirstBlock:        firstBlock,
		FirstValidCluster: firstValidCluster,
		LastValidCluster:  lastValidCluster,
		bytesPerCluster:   blocksPerCluster * blockStream.BlockSize,
	}, nil
}

func NewBasicClusterStream(
	blockStream *BlockStream,
	blocksPerCluster uint,
) (ClusterStream, error) {
	return NewClusterStream(
		blockStream,
		blocksPerCluster,
		0,
		0,
		ClusterID(blockStream.TotalBlocks/blocksPerCluster))
}

func (stream *ClusterStream) ClusterIDToBlock(clusterID ClusterID) (BlockID, error) {
	err := stream.CheckIOBounds(clusterID, 0)
	if err != nil {
		return 0, err
	}
	return stream.FirstBlock + (BlockID(uint(clusterID) * stream.BlocksPerCluster)), nil
}

func (stream *ClusterStream) CheckIOBounds(cluster ClusterID, dataLength uint) error {
	if cluster < stream.FirstValidCluster || cluster > stream.LastValidCluster {
		return fmt.Errorf(
			"invalid cluster ID %d: not in range [%d, %d]",
			cluster,
			stream.FirstValidCluster,
			stream.LastValidCluster)
	}

	if dataLength%stream.bytesPerCluster != 0 {
		return fmt.Errorf(
			"data must be a multiple of the cluster size (%d B), got %d (remainder %d)",
			stream.bytesPerCluster,
			dataLength,
			dataLength%stream.bytesPerCluster)
	}

	clusterCount := dataLength / stream.bytesPerCluster
	if uint(cluster)+clusterCount > uint(stream.LastValidCluster) {
		return fmt.Errorf(
			"cluster %d plus %d clusters of data extends past the end of the image",
			cluster,
			clusterCount)
	}

	return nil
}

func (stream *ClusterStream) Read(cluster ClusterID, count uint) ([]byte, error) {
	err := stream.CheckIOBounds(cluster, count)
	if err != nil {
		return nil, err
	}

	block, err := stream.ClusterIDToBlock(cluster)
	if err != nil {
		return nil, err
	}
	return stream.BlockStream.Read(block, count*stream.BlocksPerCluster)
}

func (stream *ClusterStream) Write(cluster ClusterID, data []byte) error {
	err := stream.CheckIOBounds(cluster, uint(len(data)))
	if err != nil {
		return err
	}

	block, err := stream.ClusterIDToBlock(cluster)
	if err != nil {
		return err
	}
	return stream.BlockStream.Write(block, data)
}
