package driver

import "github.com/dargueta/disko"

type extObjectHandle interface {
	disko.ObjectHandle
	AbsolutePath() string
}

type tExtObjectHandle struct {
	extObjectHandle
	absolutePath string
}

// wrapObjectHandle combines
func wrapObjectHandle(handle disko.ObjectHandle, absolutePath string) extObjectHandle {
	return &tExtObjectHandle{
		// FIXME (dargueta): This is hella wrong
		//extObjectHandle.ObjectHandle: handle,
		absolutePath: absolutePath,
	}
}

func (xh tExtObjectHandle) AbsolutePath() string {
	return xh.absolutePath
}
