[![License: LGPL v3](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0)
[![GoDocs](https://godocs.io/github.com/hweidner/sos?status.svg)](https://godocs.io/github.com/hweidner/sos)
[![Go Reference](https://pkg.go.dev/badge/github.com/hweidner/sos.svg)](https://pkg.go.dev/github.com/hweidner/sos)
[![Go Report Card](https://goreportcard.com/badge/github.com/hweidner/sos)](https://goreportcard.com/report/github.com/hweidner/sos)
[![Total alerts](https://img.shields.io/lgtm/alerts/g/hweidner/sos.svg?logo=lgtm&logoWidth=18)](https://lgtm.com/projects/g/hweidner/sos/alerts/)

# SOS - Simple Object Storage

An Implementation of an object storage system and API on top of a
filesystem directory

## Rationale

SOS implements an object (key/value pair) store on top on a UNIX file system.
The implementation is meant to be atomic and lock-free. SOS is intended to run
in multiple processes, even distributed among different machines, accessing a
common storage system. A common use case is to run on a Kubernetes cluster on
a persistant volume with the "many nodes read-write" property.

Besides the classic open/read/write/close operations, the only requirement to
the file system is to provide UNIX-style hard links. The file system does not
need to provide distributed/remote file locking, nor the UNIX FS semantic where
a file is not physically deleted unless the last reference to it is closed.
The supported file systems include EXT4 and XFS on local systems, and NFS and
OCFS2 as shared/distributed file systems.

## API

The following methods are provided:

* Create a new Simple Object Store.
* Store a new object (key/value pair) or overwrite an existing object.
  There are three methods to store a value from a byte slice, string, or out
  of an io.Reader.
* Get an object (value) by key.
  There are three methods to get a value into a byte slice, string, or to an
  io.Writer.
* Delete an object from the store
* Destroy a Simple Object Store entirely

## Implementation

### Internal FS structure

Keys are hashed by the hash function SHA256. The result is a hex string, e.g.

	e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

Object will be stored under a 2-level directory structure. For example, with
the key hash above, the value will be stored in the file

	root_directory/e3/b0/c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

The directories are created whenever needed first. They are never deleted, even
when all objects within the directories are deleted.

### Atomicity and Lock-Freeness

When an object is stored, it is first written to a temporary file. Open
completion of the write operation, the file will be moved to it's final
destination within the directory structure. Thus, the file appears always
atomically, regardless wether it is created or updated.

On each get operation, the file is first hardlinked to a secondary temporary
file name, before being opened, read, closed, and unlinked. Thus, the value
can always be read entirely, even if it is deleted or replaced at it' original
position. This does cost some performance, but works without locking even on
shared/distributed file systems like NFS.

## Caveats / Shortcomings

* There is no easy way to iterate over the objects, or pick a random object.
  An application needs to to keep track of the existing objects by other means,
  e.g. in an external database.
* This design does not allow to list the keys, as keys are stored hashed.
* There is no easy / atomic way to return the number of objects, or if there
  are objects in the store at all.
* The package does currently not match common interfaces like sync.Map,
  or the interface of the https://github.com/chartmuseum/storage package.

The API is currently very basic. The following API extensions might be
implemented if needed at a later time.

* Get object metadata (e.g. size or modification time)
* Rename an object (change key)
* Clone an object to another key
* Lock/Unlock object
* Read and Delete object (atomic)
* Read or Write if it does not exist (atomic)

## License

The SOS package is released under the terms of the GNU Lesser General Public
License, version 3. See the [LICENSE](LICENSE) file for details.
