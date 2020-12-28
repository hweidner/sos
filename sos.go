// (c) 2020 by Harald Weidner
//
// This library is released under the GNU Lesser General Public License
// version 3 (LGPLv3). See the LICENSE file for details.

/*
SOS, a simple object store.

Package sos implements a simple, file system based object (key/value) store.

Objects are stored in a file system directory, one file per object. The file
name is a SHA256, hex encoded hash of the key. The file's content is the
value.

For performance reasons, the files are stored in a two layer directory structure.
The subdirecories are the first and second byte of the key hash, in hex encoding.
The filename is the remainder of the hash. For example, if the hash to a given
key is

	e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

then the object will be stored in the file

	basedir/e3/b0/c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

For atomicity and lock-freeness, each new key/value pair is first stored in a
temporary file and later moved to its final position. So, there is no danger of
reading half-written values even when values are big.

The Get operation first creates a hard link to a temporary file before reading
from that file. This ensures that a Store to the same key does not affect an
already started Get operation. This is a bit slower than accessing the storage
files directly, but works lock-free even with NFS.

*/
package sos

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"
)

// SOS is the controlling data structure for the object store
type SOS struct {
	instanceID string
	base       string
}

// New creates a new simple object store at the directory path.
//
// The path must point to a location which lies on a UNIX-like file system. It
// must support the open/read/write/close methods, and UNIX-style hard links.
// The directory under path must not cross file system boundaries. If the
// directory does not exist yet, it is created upon invocation.
func New(path string) (*SOS, error) {
	if path == "" {
		return nil, fmt.Errorf("SOS: path for object storage must not be empty")
	}

	// create directory for object storage
	err := os.MkdirAll(path+"/.tmp", os.FileMode(0700))
	if err != nil {
		return nil, err
	}

	// create unique ID from hostname and random number.
	// This will be used for temporary filename creation.
	h, _ := os.Hostname()
	if h == "" {
		h = "_unknown_"
	}
	rnd := rand.Intn(1 << 32)
	id := fmt.Sprintf("%s-%08x", h, rnd)

	// Return the SOS object
	return &SOS{
		instanceID: id,
		base:       path,
	}, nil
}

// Destroy will delete an object store and remove all of its content, and the
// directory itself.
//
// Note: on NFS, this can break running Get operations.
func (s *SOS) Destroy() {
	if s.base != "" {
		_ = os.RemoveAll(s.base)
		s.base = ""
	}
}

// Store stores a key/value pair in the object store.
//
// New values are stored in a temporary file and being moved to the final
// place in order to make writes atomic and locking free.
func (s *SOS) Store(key string, value []byte) error {
	if s.base == "" {
		return fmt.Errorf("SOS: Running Store on a destroyed store")
	}

	dirname, filename := s.getpath(key)
	tmpname := s.tmpfilename()

	// write object to temporary file
	wr, err := os.OpenFile(tmpname, os.O_WRONLY|os.O_CREATE, os.FileMode(0600))
	if err != nil {
		return err
	}

	_, err = io.Copy(wr, bytes.NewReader(value))
	if err != nil {
		return err
	}

	err = wr.Close()
	if err != nil {
		return err
	}

	// create directory in storage space.
	// Note: errors are ok here, because the directory could have been created
	// by another process in the meantime
	_ = os.MkdirAll(dirname, os.FileMode(0700))

	// move object to final directory and name
	return os.Rename(tmpname, filename)
}

// Get fetches an object from tthe store, identified by the key.
//
// Value files are hardlinked to a temporary file before read, so that a
// concurrent Store/Delete operation on the same key does not break an already
// started read operation.
func (s *SOS) Get(key string) ([]byte, error) {
	if s.base == "" {
		return nil, fmt.Errorf("SOS: Running Get on a destroyed store")
	}

	_, filename := s.getpath(key)
	tmpname := s.tmpfilename()
	buffer := new(bytes.Buffer)

	// create hard link. Note that errors are ok here, as the key might not
	// exist.
	err := os.Link(filename, tmpname)
	if err != nil {
		return nil, nil
	}

	// read key from file
	fh, err := os.Open(tmpname)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(buffer, fh)
	if err != nil {
		return nil, err
	}

	err = fh.Close()
	if err != nil {
		return nil, err
	}

	// remove temporary file
	err = os.Remove(tmpname)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// Delete removes an object from the store
func (s *SOS) Delete(key string) error {
	if s.base == "" {
		return fmt.Errorf("SOS: Running Delete on a destroyed store")
	}

	_, filename := s.getpath(key)
	return os.Remove(filename)
}

// internal (unexported) helper methods

// getpath returns the directory and full path filename for a given key.
func (s *SOS) getpath(key string) (dirname, filename string) {
	h := sha256.New()
	h.Write([]byte(key))
	hs := fmt.Sprintf("%x", h.Sum(nil))

	dirname = fmt.Sprintf("%s/%c%c/%c%c", s.base, hs[0], hs[1], hs[2], hs[3])
	filename = fmt.Sprintf("%s/%s", dirname, hs[4:])
	return
}

// tmpfilename returns a temporary file name used in Store and Get
// operations
func (s *SOS) tmpfilename() string {
	tmpfname := fmt.Sprintf("%s/.tmp/%s-%d-%08x",
		s.base, s.instanceID,
		time.Now().UnixNano(),
		rand.Intn(1<<32))
	return tmpfname
}

// initialization

func init() {
	// seed the random number generator, so it delivers different random
	// numbers on each startup.
	rand.Seed(time.Now().UnixNano())
}
