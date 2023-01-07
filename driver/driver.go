package driver

import (
	"errors"
	"fmt"
	"os"
	posixpath "path"
	"path/filepath"

	"github.com/dargueta/disko"
)

// BaseDriver is an abstraction layer for all file system implementations,
// providing a single interface for interacting with them.
type BaseDriver struct {
	// Interfaces
	// disko.Driver

	implementation disko.FileSystemImplementer
	mountFlags     disko.MountFlags
	workingDirPath string
}

// New creates a new [BaseDriver] from the given implementation.
func New(
	impl disko.FileSystemImplementer,
	mountFlags disko.MountFlags,
) *BaseDriver {
	return &BaseDriver{
		implementation: impl,
		mountFlags:     mountFlags,
		workingDirPath: "/",
	}
}

func (driver *BaseDriver) NormalizePath(path string) string {
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
func (driver *BaseDriver) resolveSymlink(
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

	originalPath := object.AbsolutePath()
	currentPath := originalPath
	pathCache[currentPath] = true

	for stat.IsSymlink() {
		symlinkText, err := driver.getContentsOfObject(object)
		if err != nil {
			return nil,
				err.WithMessage(
					fmt.Sprintf(
						"can't resolve path %q, failed to read symlink %q: %s",
						originalPath,
						currentPath,
						err.Error(),
					),
				)
		}

		// Compute the next path in this chain; if it's already in the cache
		// then we hit a cycle.
		nextPath := driver.NormalizePath(string(symlinkText))
		_, exists := pathCache[nextPath]
		if exists {
			return nil,
				disko.ErrLinkCycleDetected.WithMessage(
					fmt.Sprintf(
						"found cycle resolving symlink %q: hit %q twice",
						originalPath,
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
func (driver *BaseDriver) getObjectAtPathNoFollow(
	path string,
) (extObjectHandle, disko.DriverError) {
	if path == "/" || path == "" {
		root := driver.implementation.GetRootDirectory()
		return wrapObjectHandle(root, path), nil
	}

	parentPath, baseName := posixpath.Split(path)
	parentObject, err := driver.getObjectAtPathFollowingLink(parentPath)
	if err != nil {
		return nil, err
	}

	parentStat := parentObject.Stat()
	if !parentStat.IsDir() {
		return nil,
			disko.ErrNotADirectory.WithMessage(
				fmt.Sprintf(
					"cannot resolve path %q: %q is not a directory",
					path,
					parentPath,
				),
			)
	}

	return driver.getExtObjectInDir(baseName, parentObject)
}

// getObjectAtPathFollowingLink is like [getObjectAtPathNoFollow] except that it
// always follows the last path component if it's a symlink.
func (driver *BaseDriver) getObjectAtPathFollowingLink(
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
func (driver *BaseDriver) getContentsOfObject(
	object extObjectHandle,
) ([]byte, disko.DriverError) {
	handle, err := NewFileFromObjectHandle(driver, object, disko.O_RDONLY)
	if err != nil {
		return nil, disko.ErrIOFailed.Wrap(err)
	}
	defer handle.Close()

	stat := object.Stat()
	buffer := make([]byte, int(stat.Size))

	_, readError := handle.Read(buffer)
	if readError != nil {
		return nil, disko.ErrIOFailed.Wrap(readError)
	}
	return buffer, nil
}

// getExtObjectInDir is a wrapper around [DriverImplementation.GetObject] that
// returns an [extObjectHandle].
func (driver *BaseDriver) getExtObjectInDir(
	baseName string, parentObject extObjectHandle,
) (extObjectHandle, disko.DriverError) {
	object, err := driver.implementation.GetObject(baseName, parentObject)
	if err != nil {
		return nil, err
	}

	absPath := posixpath.Join(parentObject.AbsolutePath(), baseName)
	return wrapObjectHandle(object, absPath), nil
}

// createExtObject is a wrapper around [DriverImplementation.CreateObject] that
// returns an [extObjectHandle].
func (driver *BaseDriver) createExtObject(
	baseName string, parentObject extObjectHandle, perm os.FileMode,
) (extObjectHandle, disko.DriverError) {
	rawObject, err := driver.implementation.CreateObject(
		baseName,
		parentObject,
		perm,
	)
	if err != nil {
		return nil, err
	}

	absPath := posixpath.Join(parentObject.AbsolutePath(), baseName)
	object := wrapObjectHandle(rawObject, absPath)
	return object, nil
}

// OpenFile opens a file for I/O.
func (driver *BaseDriver) OpenFile(
	path string,
	flags disko.IOFlags,
	perm os.FileMode,
) (File, error) {
	absPath := driver.NormalizePath(path)
	ioFlags := disko.IOFlags(flags)

	if ioFlags.RequiresWritePerm() && !driver.mountFlags.CanWrite() {
		return File{}, disko.ErrReadOnlyFileSystem.WithMessage(
			fmt.Sprintf(
				"can't open %q for writing: image is mounted read-only",
				absPath,
			),
		)
	}

	object, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		// An error occurred. If the file is missing we may be able to create it
		// and proceed.
		if errors.Is(err, disko.ErrNotFound) {
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
				object, err = driver.createExtObject(
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
		return File{}, disko.ErrIsADirectory.WithMessage(absPath)
	}

	return NewFileFromObjectHandle(driver, object, flags)
}

func (driver *BaseDriver) Chdir(path string) error {
	absPath := driver.NormalizePath(path)

	object, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		return err
	}
	return driver.chdirToObject(object)
}

// chdirToObject is like [baseDriver.Chdir] except it uses an object.
func (driver *BaseDriver) chdirToObject(object extObjectHandle) error {
	absPath := object.AbsolutePath()
	stat := object.Stat()
	if !stat.IsDir() {
		return disko.ErrNotADirectory.WithMessage(absPath)
	}

	driver.workingDirPath = absPath
	return nil
}

func (driver *BaseDriver) Open(path string) (File, error) {
	return driver.OpenFile(path, disko.O_RDONLY, 0)
}

func (driver *BaseDriver) ReadFile(path string) ([]byte, error) {
	path = driver.NormalizePath(path)

	object, err := driver.getObjectAtPathFollowingLink(path)
	if err != nil {
		return nil, err
	}
	return driver.getContentsOfObject(object)
}

func (driver *BaseDriver) SameFile(fi1, fi2 os.FileInfo) bool {
	stat1 := fi1.Sys().(disko.FileStat)
	stat2 := fi2.Sys().(disko.FileStat)
	return stat1.InodeNumber == stat2.InodeNumber
}

func (driver *BaseDriver) Stat(path string) (disko.FileStat, error) {
	path = driver.NormalizePath(path)

	object, err := driver.getObjectAtPathFollowingLink(path)
	if err != nil {
		return disko.FileStat{}, err
	}
	return object.Stat(), nil
}

func (driver *BaseDriver) ReadDir(path string) ([]disko.DirectoryEntry, error) {
	absPath := driver.NormalizePath(path)

	directory, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		return nil, err
	}
	return driver.readDir(directory)
}

// readDir implements [ReadDir] for any directory object handle.
func (driver *BaseDriver) readDir(
	directory extObjectHandle,
) ([]disko.DirectoryEntry, error) {
	direntNames, err := directory.(disko.SupportsListDirHandle).ListDir()
	if err != nil {
		return nil, err
	}

	output := make([]disko.DirectoryEntry, 0, len(direntNames))
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

func (driver *BaseDriver) Readlink(path string) (string, error) {
	if !driver.implementation.GetFSFeatures().HasSymbolicLinks {
		return "", disko.ErrNotSupported
	}

	path = driver.NormalizePath(path)
	object, err := driver.getObjectAtPathNoFollow(path)
	if err != nil {
		return "", err
	}

	stat := object.Stat()
	if !stat.IsSymlink() {
		return "", disko.ErrInvalidArgument.WithMessage(
			fmt.Sprintf("%q is not a symlink", path),
		)
	}

	contents, err := driver.getContentsOfObject(object)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}

func (driver *BaseDriver) Lstat(path string) (disko.FileStat, error) {
	path = driver.NormalizePath(path)
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

// Create creates a file and opens it for reading and writing. It fails if the
// file already exists.
func (driver *BaseDriver) Create(path string) (File, error) {
	return driver.OpenFile(
		path,
		disko.O_RDWR|disko.O_CREATE|disko.O_EXCL,
		0,
	)
}

// removeDotsFromSlice returns a copy of `arr`, filtering out "." and "..". If
// neither are in the slice, it returns `arr` unmodified.
func removeDotsFromSlice(arr []string) []string {
	numToIgnore := 0

	for _, element := range arr {
		if element == "." || element == ".." {
			numToIgnore++
		}
	}

	// If we didn't find . or .. anywhere then don't bother copying the slice,
	// and return it unmodified.
	if numToIgnore == 0 {
		return arr
	}

	// We found at least one dot string. Create a copy of the slice while skipping
	// over the things we don't want.
	newSlice := make([]string, 0, len(arr)-numToIgnore)
	for _, element := range arr {
		if element != "." && element != ".." {
			newSlice = append(newSlice, element)
		}
	}
	return newSlice
}

func (driver *BaseDriver) Remove(path string) error {
	absPath := driver.NormalizePath(path)
	object, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		return err
	}

	stat := object.Stat()
	if stat.IsDir() {
		// Caller wants to remove a directory. The directory must be empty, i.e.
		// must at most only contain the "." and ".." entries.
		names, err := object.(disko.SupportsListDirHandle).ListDir()
		if err != nil {
			return err
		}

		// Remove the "." and ".." since we don't care about them.
		names = removeDotsFromSlice(names)

		// If there are any other entries in here the directory isn't empty and
		// we must fail.
		if len(names) > 0 {
			return disko.ErrDirectoryNotEmpty.WithMessage(
				fmt.Sprintf("can't remove %q: directory not empty", absPath),
			)
		}
	} else if !stat.IsFile() {
		return disko.ErrInvalidArgument.WithMessage(
			fmt.Sprintf("can't remove %q: not a file or directory", absPath),
		)
	}

	return object.Unlink()
}

// Truncate sets the size of a file to 0.
func (driver *BaseDriver) Truncate(path string) error {
	absPath := driver.NormalizePath(path)
	object, err := driver.getObjectAtPathFollowingLink(absPath)
	if err != nil {
		return err
	}

	stat := object.Stat()
	if stat.IsDir() {
		return disko.ErrIsADirectory.WithMessage(absPath)
	}
	return object.Resize(0)
}

// WriteFile sets the contents of a file to the given data, creating it if
// necessary.
func (driver *BaseDriver) WriteFile(
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

func (driver *BaseDriver) Mkdir(path string, perm os.FileMode) error {
	// Force the permissions flags to indicate this is a directory so that the
	// caller doesn't have to remember to do it themselves.
	perm &^= os.ModeType
	perm |= os.ModeDir

	absPath := driver.NormalizePath(path)
	parentDir, baseName := posixpath.Split(absPath)

	parentObject, err := driver.getObjectAtPathFollowingLink(parentDir)
	if err != nil {
		return err
	}

	parentStat := parentObject.Stat()
	if !parentStat.IsDir() {
		return disko.ErrNotADirectory.WithMessage(
			fmt.Sprintf(
				"cannot create %q: %q is not a directory",
				absPath,
				parentDir,
			),
		)
	}

	_, err = driver.implementation.CreateObject(baseName, parentObject, perm)
	return err
}

func (driver *BaseDriver) MkdirAll(path string, perm os.FileMode) error {
	absPath := driver.NormalizePath(path)
	parentDir, baseName := posixpath.Split(absPath)

	// Force the permissions flags to indicate this is a directory so that the
	// caller doesn't have to remember to do it themselves.
	perm &^= os.ModeType
	perm |= os.ModeDir

	parentObject, err := driver.getObjectAtPathFollowingLink(parentDir)
	if err != nil {
		if errors.Is(err, disko.ErrNotFound) {
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

func (driver *BaseDriver) RemoveAll(path string) error {
	path = driver.NormalizePath(path)
	directory, err := driver.getObjectAtPathFollowingLink(path)
	if err != nil {
		return err
	}

	stat := directory.Stat()
	if !stat.IsDir() {
		return disko.ErrNotADirectory.WithMessage(path)
	}

	// Block an attempt at `rm -rf /`, because some clown is gonna try it.
	root := driver.implementation.GetRootDirectory()
	if root.SameAs(directory) {
		return disko.ErrPermissionDenied.WithMessage(
			"you can't remove the root directory",
		)
	}

	return driver.removeDirectory(directory)
}

// removeDirectory is equivalent to `rm -rf` for a directory handle.
//
// Deletion is depth-first, and terminates on the first error encountered.
// Ownership and other permissions are not checked.
func (driver *BaseDriver) removeDirectory(directory extObjectHandle) error {
	var err error

	direntNames, err := directory.(disko.SupportsListDirHandle).ListDir()
	if err != nil {
		return err
	}

	for _, name := range direntNames {
		if name == "." || name == ".." {
			continue
		}

		dirent, err := driver.getExtObjectInDir(name, directory)
		if err != nil {
			return err
		}

		direntStat := dirent.Stat()

		// If this is a directory, recursively delete its contents.
		if direntStat.IsDir() {
			absPath := posixpath.Join(directory.AbsolutePath(), name)
			wrappedDirent := wrapObjectHandle(dirent, absPath)
			rmErr := driver.removeDirectory(wrappedDirent)
			if rmErr != nil {
				return rmErr
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

// Getwd returns the working directory as an absolute path. The error will always
// be nil; it's only there for compatibility with [os.Getwd].
func (driver *BaseDriver) Getwd() (string, error) {
	return driver.workingDirPath, nil
}

func (driver *BaseDriver) GetFSFeatures() disko.FSFeatures {
	return driver.implementation.GetFSFeatures()
}
