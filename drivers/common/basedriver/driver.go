package basedriver

import (
	"fmt"
	"io"
	"os"
	posixpath "path"
	"path/filepath"
	"time"

	"github.com/dargueta/disko"
	"github.com/dargueta/disko/drivers/common"
	"golang.org/x/exp/slices"
)

////////////////////////////////////////////////////////////////////////////////

// DriverImplementation is an interface that drivers must implement to get all
// default functionality from the CommonDriver.
type DriverImplementation interface {
	// CreateObject creates an object on the file system that is *not* a
	// directory. The following guarantees apply: A) this will never be called
	// for an object that already exists; B) `parent` will always be a valid
	// object handle.
	CreateObject(
		name string,
		parent ObjectHandle,
		perm os.FileMode,
	) (ObjectHandle, disko.DriverError)

	// GetObject returns a handle to an object with the given name in a directory
	// specified by `parent`. The following guarantees apply: A) this will never
	// be called for a nonexistent object; B) `parent` will always be a valid
	// object handle.
	GetObject(
		name string,
		parent ObjectHandle,
	) (ObjectHandle, disko.DriverError)

	// GetRootDirectory returns a handle to the root directory of the disk image.
	// This must always be a valid object handle, even if directories are not
	// supported by the file system (e.g. FAT8).
	GetRootDirectory() ObjectHandle

	// MarkFileClosed is a provisional function and should be ignored for the
	// time being.
	MarkFileClosed(file *File) disko.DriverError

	// FSStat returns information about the file system. Multiple calls to this
	// function should return identical data if no modifications have been made
	// to the file system.
	FSStat() disko.FSStat

	GetFSFeatures() disko.FSFeatures

	FormatImage(
		image io.ReadWriteSeeker,
		totalSize int64,
	) error
}

type CommonDriver struct {
	implementation DriverImplementation
	mountFlags     disko.MountFlags
	workingDirPath string
}

func NewDriver(implementation DriverImplementation, mountFlags disko.MountFlags) *CommonDriver {
	return &CommonDriver{
		implementation: implementation,
		mountFlags:     mountFlags,
		workingDirPath: "/",
	}
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
// isn't a symlink, this becomes a no-op and returns the handle unmodified.
func (driver *CommonDriver) resolveSymlink(
	object extObjectHandle,
) (extObjectHandle, disko.DriverError) {
	stat := object.Stat()
	if !stat.IsSymlink() {
		return object, nil
	}

	// Symbolic links can result in cycles, so we need to keep track of all the
	// paths we visit. If we resolve a symlink to a path that's already in the
	// dictionary, we found a loop and must fail.
	pathCache := make(map[string]bool)

	currentPath := object.AbsolutePath()
	pathCache[currentPath] = true

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

		// Compute the next path in this chain; if it's already in the cache
		// then we hit a cycle.
		nextPath := driver.normalizePath(string(symlinkText))
		_, exists := pathCache[nextPath]
		if exists {
			return nil,
				disko.NewDriverErrorWithMessage(
					disko.ELOOP,
					fmt.Sprintf(
						"found cycle resolving symlink `%s`: hit `%s` twice",
						path,
						nextPath,
					),
				)
		}

		// Get the object at the next path but don't dereference it.
		object, err = driver.getObjectAtPathNoFollow(nextPath)
		if err != nil {
			return nil, err
		}

		// Update the latest path and file stats for the next loop iteration.
		stat = object.Stat()
		currentPath = nextPath
	}

	return object, nil
}

// getObjectAtPathNoFollow resolves a normalized absolute path to an object
// handle. It follows symbolic links for intermediate directories, but does *not*
// follow the final path component if it's a symbolic link.
//
// `path` must be a normalized absolute path.
func (driver *CommonDriver) getObjectAtPathNoFollow(
	path string,
) (extObjectHandle, disko.DriverError) {
	if path == "/" || path == "" {
		return driver.implementation.GetRootDirectory(), nil
	}

	parentPath, baseName := posixpath.Split(path)
	parentObject, err := driver.getObjectAtPathFollowingLink(parentPath)
	if err != nil {
		return nil, err
	}

	parentStat := parentObject.Stat()
	if !parentStat.IsDir() {
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

	object := driver.implementation.GetObject(baseName, parentObject)
	return wrapObjectHandle(object, path), nil
}

func (driver *CommonDriver) getObjectAtPathFollowingLink(
	path string,
) (extObjectHandle, disko.DriverError) {
	object, err := driver.getObjectAtPathNoFollow(path)
	if err != nil {
		return nil, err
	}

	stat := object.Stat()
	for stat.IsSymlink() {
		object, err = driver.resolveSymlink(object)
		if err != nil {
			return nil, err
		}
		stat = object.Stat()
	}

	return object, nil
}

// getContentsOfObject returns the contents of an object as it exists on the
// file system, regardless of whether it's a file or directory. Symbolic links
// are not followed.
func (driver *CommonDriver) getContentsOfObject(
	object extObjectHandle,
) ([]byte, disko.DriverError) {
	handle, err := NewFileFromObjectHandle(driver, object, disko.O_RDONLY)
	if err != nil {
		return nil, disko.NewDriverErrorFromError(disko.EIO, err)
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
	flags disko.IOFlags,
	perm os.FileMode,
) (File, error) {
	absPath := driver.normalizePath(path)
	ioFlags := disko.IOFlags(flags)

	if ioFlags.RequiresWritePerm() && !driver.mountFlags.CanWrite() {
		return File{}, disko.NewDriverErrorWithMessage(
			disko.EROFS,
			fmt.Sprintf(
				"can't open `%s` for writing: image is mounted read-only",
				absPath,
			),
		)
	}

	object, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		// An error occurred. If the file is missing we may be able to create it
		// and proceed.
		if err.Errno() == disko.ENOENT {
			// File does not exist, can we create it?
			if ioFlags.Create() {
				// To create the missing file, we need to get a handle for its
				// parent directory, then call CreateObject() for the file in
				// that directory.
				parentDir, baseName := posixpath.Split(absPath)
				parentObject, err := driver.getObjectAtPathFollowingLink(parentDir)
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
	if !stat.IsFile() {
		return File{},
			disko.NewDriverErrorWithMessage(
				disko.EISDIR,
				fmt.Sprintf("`%s` isn't a regular file", absPath),
			)
	}

	return NewFileFromObjectHandle(driver, object, flags)
}

// ReadingDriver ---------------------------------------------------------------

func (driver *CommonDriver) Chdir(path string) error {
	absPath := driver.normalizePath(path)

	object, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		return err
	}
	return driver.chdirToObject(object, absPath)
}

func (driver *CommonDriver) chdirToObject(object extObjectHandle) error {
	absPath := object.AbsolutePath()
	stat := object.Stat()
	if !stat.IsDir() {
		return disko.NewDriverErrorWithMessage(
			disko.ENOTDIR,
			fmt.Sprintf("not a directory: `%s`", absPath),
		)
	}

	driver.workingDirPath = absPath
	return nil
}

func (driver *CommonDriver) Open(path string) (File, error) {
	return driver.OpenFile(path, disko.O_RDONLY, 0)
}

func (driver *CommonDriver) ReadFile(path string) ([]byte, error) {
	path = driver.normalizePath(path)

	object, err := driver.getObjectAtPathFollowingLink(path)
	if err != nil {
		return nil, err
	}
	return driver.getContentsOfObject(object)
}

func (driver *CommonDriver) SameFile(fi1, fi2 os.FileInfo) bool {
	stat1 := fi1.Sys().(disko.FileStat)
	stat2 := fi2.Sys().(disko.FileStat)
	return stat1.InodeNumber == stat2.InodeNumber
}

func (driver *CommonDriver) Stat(path string) (disko.FileStat, error) {
	path = driver.normalizePath(path)

	object, err := driver.getObjectAtPathFollowingLink(path)
	if err != nil {
		return disko.FileStat{}, err
	}
	return object.Stat(), nil
}

// DirReadingDriver ------------------------------------------------------------

func (driver *CommonDriver) ReadDir(path string) ([]disko.DirectoryEntry, error) {
	absPath := driver.normalizePath(path)

	directory, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		return nil, err
	}

	direntNames, err := directory.ListDir()
	if err != nil {
		return nil, err
	}

	output := make([]disko.DirectoryEntry, 0, len(dirents))
	for _, name := range direntNames {
		// Ignore "." and ".." entries if present
		if name == "." || name == ".." {
			continue
		}

		direntObject, err := driver.implementation.GetObject(name, directory)
		if err != nil {
			return output, err
		}

		dirent := NewDirectoryEntryFromHandle(direntObject)
		output = append(output, dirent)
	}

	return output, nil
}

// ReadingLinkingDriver --------------------------------------------------------

func (driver *CommonDriver) Readlink(path string) (string, error) {
	path = driver.normalizePath(path)
	object, err := driver.getObjectAtPathNoFollow(path)
	if err != nil {
		return "", err
	}

	stat := object.Stat()
	if !stat.IsSymlink() {
		return "", disko.NewDriverErrorWithMessage(
			disko.EINVAL,
			fmt.Sprintf("`%s` is not a symlink", path),
		)
	}

	contents, err := driver.getContentsOfObject(object)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}

func (driver *CommonDriver) Lstat(path string) (disko.FileStat, error) {
	path = driver.normalizePath(path)
	object, err := driver.getObjectAtPathNoFollow(path)
	if err != nil {
		return disko.FileStat{}, err
	}

	// Unconditionally try to resolve `object` as a symlink. If it isn't one,
	// nothing happens and we get `object` back.
	object, err = driver.resolveSymlink(object)
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

func removeDotsFromSlice(arr []string) []string {
	toRemove := []string{".", ".."}

	for _, dotStr := range toRemove {
		index := slices.Index(arr, dotStr)
		if index >= 0 {
			arr = slices.Delete(arr, index, 1)
		}
	}
	return slices.Clip(arr)
}

func (driver *CommonDriver) Remove(path string) error {
	absPath := driver.normalizePath(path)
	object, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		return err
	}

	stat := object.Stat()
	if stat.IsDir() {
		// Caller wants to remove a directory. The directory must be empty, i.e.
		// must at most only contain the "." and ".." entries.
		names, err := object.ListDir()
		if err != nil {
			return err
		}

		// Remove the "." and ".." since we don't care about them.
		names = removeDotsFromSlice(names)

		// If there are any other entries in here the directory isn't empty and
		// we must fail.
		if len(dirents) > 0 {
			return disko.NewDriverErrorWithMessage(
				disko.ENOTEMPTY,
				fmt.Sprintf("can't remove `%s`: directory not empty", absPath),
			)
		}
	} else if !stat.IsFile() {
		return disko.NewDriverErrorWithMessage(
			disko.EINVAL,
			fmt.Sprintf("can't remove `%s`: not a file or directory", absPath),
		)
	}

	return object.Unlink()
}

func (driver *CommonDriver) Truncate(path string) error {
	absPath := driver.normalizePath(path)
	object, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		return err
	}
	return object.Resize(0)
}

func (driver *CommonDriver) WriteFile(
	path string,
	data []byte,
	perm os.FileMode,
) error {
	handle, err := driver.OpenFile(
		path,
		disko.O_WRONLY|disko.O_CREATE|disko.O_TRUNC,
		perm,
	)
	if err != nil {
		return err
	}
	defer handle.Close()

	_, err = handle.Write(data)
	return err
}

// DirWritingDriver ------------------------------------------------------------

func (driver *CommonDriver) Mkdir(path string, perm os.FileMode) error {
	// Force the permissions flags to indicate this is a directory
	perm &^= os.ModeType
	perm |= os.ModeDir

	absPath := driver.normalizePath(path)
	parentDir, baseName := posixpath.Split(absPath)

	parentObject, err := driver.getObjectAtPathFollowingLink(parentDir)
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

	parentObject, err := driver.getObjectAtPathFollowingLink(parentDir)
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
	directory, err := driver.getObjectAtPathFollowingLink(path)
	if err != nil {
		return err
	}

	stat := directory.Stat()
	if !stat.IsDir() {
		return disko.NewDriverErrorWithMessage(
			disko.ENOTDIR,
			fmt.Sprintf("cannot remove `%s`: not a directory", path),
		)
	}

	return driver.removeDirectory(directory)
}


// removeDirectory is equivalent to `rm -rf` for a directory handle.
//
// Deletion is depth-first, and terminates on the first error encountered.
// Ownership and other permissions are not checked.
func (driver *CommonDriver) removeDirectory(directory extObjectHandle) error {
	var err error

	direntNames, err := directory.ListDir()
	if err != nil {
		return err
	}

	for _, name := range direntNames {
		if name == "." || name == ".." {
			continue
		}

		dirent, err := driver.implementation.GetObject(directory, name)
		direntStat := dirent.Stat()

		// If this is a directory, recursively delete its contents.
		if direntStat.IsDir() {
			err = driver.removeDirectory(dirent)
			if err != nil {
				return err
			}
		}

		// Delete the file or empty directory.
		err = dirent.Unlink()
		if err != nil {
			return err
		}
	}

	return nil
}
