package basedriver

import (
	"fmt"
	"os"
	posixpath "path"
	"path/filepath"

	"github.com/dargueta/disko"
)

////////////////////////////////////////////////////////////////////////////////

type ObjectDescriptor interface {
	ID() int64
	Stat() disko.FileStat
	Open(flags int) (disko.File, disko.DriverError)
	Resize(newSize int64) disko.DriverError
	Unlink() disko.DriverError
	Update(stat disko.FileStat) disko.DriverError
	ListDir() (map[string]ObjectDescriptor, disko.DriverError)
	Name() string
}

// DriverImplementation is an interface that drivers must implement to get all
// default functionality from the CommonDriver.
type DriverImplementation interface {
	// CreateObject creates an object on the file system that is *not* a
	// directory. This is guaranteed to never be called
	CreateObject(
		name string,
		parent ObjectDescriptor,
		perm os.FileMode,
	) (ObjectDescriptor, disko.DriverError)

	GetObject(
		name string,
		parent ObjectDescriptor,
	) (ObjectDescriptor, disko.DriverError)

	GetRootDirectory() ObjectDescriptor

	MarkFileClosed(file *File) disko.DriverError
	FSStat() disko.FSStat
}

type CommonDriver struct {
	/*
		OpeningDriver:	DONE
			OpenFile 	DONE

		ReadingDriver:	DONE
			Open 		DONE
			ReadFile	DONE
			SameFile	DONE
			Stat		DONE

		DirReadingDriver:	DONE
			ReadDir			DONE

		ReadingLinkingDriver:	DONE
			Lstat			DONE
			Readlink		DONE

		WritingDriver:
			Chmod
			Chown
			Chtimes
			Create			DONE
			Flush
			Remove
			Repath
			Truncate		DONE
			WriteFile

		DirWritingDriver:	DONE
			Mkdir			DONE
			MkdirAll		DONE
			RemoveAll		DONE

		WritingLinkingDriver:
			Lchown
			Link
			Symlink
	*/

	implementation DriverImplementation
	mountFlags     disko.MountFlags
	workingDirPath string
}

func (driver *CommonDriver) normalizePath(path string) string {
	path = posixpath.Clean(filepath.ToSlash(path))
	if path == "." {
		path = "/"
	}
	if posixpath.IsAbs(path) {
		return path
	}
	return posixpath.Join(driver.workingDirPath, path)
}

// resolveSymlink dereferences `object` (if it's a symlink), following multiple
// levels of indirection if needed to get to a file  system object. If `object`
// isn't a symlink, this becmes a no-op and returns it unmodified.
func (driver *CommonDriver) resolveSymlink(
	object ObjectDescriptor,
	path string,
) (ObjectDescriptor, disko.DriverError) {
	stat := object.Stat()
	if !stat.IsSymlink() {
		return object, nil
	}

	// Symbolic links can result in cycles, so we need to keep track of all the
	// paths we visit. If we resolve a symlink to a path that's already in the
	// dictionary, we found a loop and must fail.
	pathCache := make(map[string]bool)
	pathCache[path] = true

	currentPath := path
	for stat.IsSymlink() {
		symlinkText, err := driver.getContentsOfObject(object)
		if err != nil {
			return nil,
				disko.NewDriverErrorWithMessage(
					err.Errno(),
					fmt.Sprintf(
						"can't resolve path `%s`, failed to read symlink `%s`: %s",
						path,
						currentPath,
						err.Error(),
					),
				)
		}

		nextPath := driver.normalizePath(string(symlinkText))
		_, exists := pathCache[nextPath]
		if exists {
			return nil,
				disko.NewDriverErrorWithMessage(
					disko.ELOOP,
					fmt.Sprintf(
						"found loop resolving symlink `%s`: hit `%s` twice",
						path,
						nextPath,
					),
				)
		}

		object, err = driver.getObjectAtPath(nextPath)
		if err != nil {
			return nil, err
		}

		stat = object.Stat()
		currentPath = nextPath
	}

	return object, nil
}

// getObjectAtPath resolves a normalized absolute path to an object descriptor.
// It follows symbolic links for directories, but does *not* follow the final
// path component if it's a symbolic link.
func (driver *CommonDriver) getObjectAtPath(
	path string,
) (ObjectDescriptor, disko.DriverError) {
	if path == "/" || path == "" {
		return driver.implementation.GetRootDirectory(), nil
	}

	parentPath, baseName := posixpath.Split(path)
	parentObject, err := driver.getObjectAtPath(parentPath)
	if err != nil {
		return nil, err
	}

	parentStat := parentObject.Stat()
	if parentStat.IsSymlink() {
		// The parent directory is a symbolic link to somewhere. Resolve it.
		parentObject, err = driver.resolveSymlink(parentObject, parentPath)
		if err != nil {
			return nil, err
		}

		parentStat = parentObject.Stat()
	}

	if parentStat.IsDir() {
		return driver.implementation.GetObject(baseName, parentObject)
	}

	return nil,
		disko.NewDriverErrorWithMessage(
			disko.ENOTDIR,
			fmt.Sprintf(
				"cannot resolve path `%s`: `%s` is not a directory",
				path,
				parentPath,
			),
		)
}

// getContentsOfObject returns the contents of an object as it exists on the
// file system, regardless of whether it's a file or directory. Symbolic links
// are not followed.
func (driver *CommonDriver) getContentsOfObject(
	object ObjectDescriptor,
) ([]byte, disko.DriverError) {
	handle, err := object.Open(disko.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	stat := object.Stat()
	buffer := make([]byte, int(stat.Size))

	_, readError := handle.Read(buffer)
	if readError != nil {
		return nil, disko.NewDriverErrorFromError(disko.EIO, readError)
	}
	return buffer, nil
}

// OpeningDriver ---------------------------------------------------------------

func (driver *CommonDriver) OpenFile(
	path string,
	flags int,
	perm os.FileMode,
) (File, error) {
	absPath := driver.normalizePath(path)
	ioFlags := disko.IOFlags(flags)

	object, err := driver.getObjectAtPath(absPath)
	if err != nil {
		// An error occurred. If the file is missing we may be able to create it
		// and proceed.
		if err.Errno() == disko.ENOENT {
			// File does not exist, can we create it?
			if ioFlags.Create() {
				// To create the missing file, we need to get a descriptor for
				// its parent directory, then call CreateObject() for the file
				// in that directory.
				parentDir, baseName := posixpath.Split(absPath)
				parentObject, err := driver.getObjectAtPath(parentDir)
				if err != nil {
					// Parent directory doesn't exist
					return File{}, err
				}
				object, err = driver.implementation.CreateObject(
					baseName,
					parentObject,
					perm,
				)
			}
			// Else: The file doesn't exist and we can't create it.
		}
		// Else: the problem isn't that the file doesn't exist.

		// If we haven't resolved the error, fail.
		if err != nil {
			return File{}, err
		}
	}

	stat := object.Stat()
	if stat.IsSymlink() {
		// The file is a symbolic link. Resolve it.
		object, err = driver.resolveSymlink(object, absPath)
		if err != nil {
			return File{}, err
		}

		// Update `stat` to contain the resolved object's information, not that
		// of the symlink.
		stat = object.Stat()
	}

	if !stat.IsFile() {
		return File{},
			disko.NewDriverErrorWithMessage(
				disko.ENFILE,
				fmt.Sprintf("`%s` isn't a regular file", absPath),
			)
	}

	baseFileObject, err := object.Open(flags)
	if err != nil {
		return File{}, err
	}

	return File{
		File:         baseFileObject,
		owningDriver: driver,
		fileInfo: FileInfo{
			FileStat: stat,
			name:     posixpath.Base(absPath),
		},
		ioFlags: ioFlags,
	}, nil
}

// ReadingDriver ---------------------------------------------------------------

func (driver *CommonDriver) Open(path string) (File, error) {
	return driver.OpenFile(path, disko.O_RDONLY, 0)
}

func (driver *CommonDriver) ReadFile(path string) ([]byte, error) {
	path = driver.normalizePath(path)
	object, err := driver.getObjectAtPath(path)
	if err != nil {
		return nil, err
	}

	handle, err := object.Open(disko.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	buffer := make([]byte, object.Stat().Size)
	_, readErr := handle.Read(buffer)
	return buffer, readErr
}

func (driver *CommonDriver) SameFile(fi1, fi2 os.FileInfo) bool {
	stat1 := fi1.Sys().(disko.FileStat)
	stat2 := fi2.Sys().(disko.FileStat)
	return stat1.InodeNumber == stat2.InodeNumber
}

func (driver *CommonDriver) Stat(path string) (disko.FileStat, error) {
	path = driver.normalizePath(path)
	object, err := driver.getObjectAtPath(path)
	if err != nil {
		return disko.FileStat{}, err
	}
	return object.Stat(), nil
}

// DirReadingDriver ------------------------------------------------------------

func (driver *CommonDriver) ReadDir(path string) ([]disko.DirectoryEntry, error) {
	path = driver.normalizePath(path)
	object, err := driver.getObjectAtPath(path)
	if err != nil {
		return nil, err
	}

	dirents, err := object.ListDir()
	if err != nil {
		return nil, err
	}

	output := make([]disko.DirectoryEntry, 0, len(dirents))
	for _, direntObject := range dirents {
		dirent := NewDirectoryEntryFromDescriptor(direntObject)
		output = append(output, dirent)
	}

	return output, nil
}

// ReadingLinkingDriver --------------------------------------------------------

func (driver *CommonDriver) Readlink(path string) (string, error) {
	path = driver.normalizePath(path)
	object, err := driver.getObjectAtPath(path)
	if err != nil {
		return "", err
	}

	contents, err := driver.getContentsOfObject(object)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}

func (driver *CommonDriver) Lstat(path string) (disko.FileStat, error) {
	path = driver.normalizePath(path)
	object, err := driver.getObjectAtPath(path)
	if err != nil {
		return disko.FileStat{}, err
	}

	// Unconditionally try to resolve `object` as a symlink. If it isn't one,
	// nothing happens and we get `object` back.
	object, err = driver.resolveSymlink(object, path)
	if err != nil {
		return disko.FileStat{}, err
	}
	return object.Stat(), nil
}

// WritingDriver ---------------------------------------------------------------

func (driver *CommonDriver) Create(path string) (File, error) {
	return driver.OpenFile(
		path,
		disko.O_RDWR|disko.O_CREATE|disko.O_EXCL,
		0,
	)
}

func (driver *CommonDriver) Truncate(path string) error {
	absPath := driver.normalizePath(path)
	object, err := driver.getObjectAtPath(absPath)
	if err != nil {
		return err
	}
	return object.Resize(0)
}

// DirWritingDriver ------------------------------------------------------------

func (driver *CommonDriver) Mkdir(path string, perm os.FileMode) error {
	// Force the permissions flags to indicate this is a directory
	perm &^= os.ModeType
	perm |= os.ModeDir

	absPath := driver.normalizePath(path)
	parentDir, baseName := posixpath.Split(absPath)

	parentObject, err := driver.getObjectAtPath(parentDir)
	if err != nil {
		return err
	}

	parentStat := parentObject.Stat()
	if !parentStat.IsDir() {
		return disko.NewDriverErrorWithMessage(
			disko.ENOTDIR,
			fmt.Sprintf(
				"cannot create `%s`: `%s is not a directory",
				absPath,
				parentDir,
			),
		)
	}

	_, err = driver.implementation.CreateObject(baseName, parentObject, perm)
	return err
}

func (driver *CommonDriver) MkdirAll(path string, perm os.FileMode) error {
	absPath := driver.normalizePath(path)
	parentDir, baseName := posixpath.Split(absPath)

	// Force the permissions flags to indicate this is a directory
	perm &^= os.ModeType
	perm |= os.ModeDir

	parentObject, err := driver.getObjectAtPath(parentDir)
	if err != nil {
		if err.Errno() == disko.ENOENT {
			// Parent directory doesn't exist, create it.
			driver.MkdirAll(parentDir, perm)
		} else {
			// Different error, we won't handle it.
			return err
		}
	}

	_, err = driver.implementation.CreateObject(baseName, parentObject, perm)
	return err
}

func (driver *CommonDriver) RemoveAll(path string) error {
	path = driver.normalizePath(path)
	object, err := driver.getObjectAtPath(path)
	if err != nil {
		return err
	}

	stat := object.Stat()
	if !stat.IsDir() {
		return disko.NewDriverErrorWithMessage(
			disko.ENOTDIR,
			fmt.Sprintf("cannot remove `%s`: not a directory", path),
		)
	}

	direntMap, err := object.ListDir()
	if err != nil {
		return err
	}

	for name, dirent := range direntMap {
		if name == "." || name == ".." {
			continue
		}

		direntStat := dirent.Stat()
		direntPath := posixpath.Join(path, name)

		var rmErr error
		if direntStat.IsDir() {
			rmErr = driver.RemoveAll(direntPath)
			if rmErr != nil {
				return rmErr
			}
		}

		rmErr = dirent.Unlink()
		if rmErr != nil {
			return rmErr
		}
	}

	return nil
}
