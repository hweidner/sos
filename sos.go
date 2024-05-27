// (c) 2020-2021 by Harald Weidner
//
// This library is released under the Mozilla License, version 2 (MPLv2).
// See the LICENSE file for details.

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
	"strings"
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
	err := os.MkdirAll(path+"/.tmp", os.FileMode(0o700))
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

// Store stores a key/value pair, given as string and byte slice, in the
// object store.
func (s *SOS) Store(key string, value []byte) error {
	return s.StoreFrom(key, bytes.NewReader(value))
}

// StoreString stores a key/value pair, given as strings, in the object store.
func (s *SOS) StoreString(key, value string) error {
	return s.StoreFrom(key, strings.NewReader(value))
}

// StoreFrom stores a value, which is read from an io.Reader, under the given
// key in the object store.
func (s *SOS) StoreFrom(key string, rd io.Reader) error {
	if s.base == "" {
		return fmt.Errorf("SOS: Running Store on a destroyed store")
	}

	dirname, filename := s.getpath(key)
	tmpname := s.tmpfilename()

	// write object to temporary file
	wr, err := os.OpenFile(tmpname, os.O_WRONLY|os.O_CREATE, os.FileMode(0o600))
	if err != nil {
		return err
	}

	_, err = io.Copy(wr, rd)
	if err != nil {
		_ = os.Remove(tmpname)
		return err
	}

	err = wr.Close()
	if err != nil {
		_ = os.Remove(tmpname)
		return err
	}

	// create directory in storage space.
	// Note: errors are ok here, because the directory could have been created
	// by another process in the meantime
	_ = os.MkdirAll(dirname, os.FileMode(0o700))

	// move object to final directory and name
	return os.Rename(tmpname, filename)
}

// Get fetches an object from the store, identified by the key, and returns
// it as byte slice.
func (s *SOS) Get(key string) ([]byte, error) {
	buffer := new(bytes.Buffer)

	err := s.GetTo(key, buffer)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// GetString fetches an object from the store, identified by the key, and returns
// it as a string
func (s *SOS) GetString(key string) (string, error) {
	var buffer strings.Builder

	err := s.GetTo(key, &buffer)
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}

// GetTo fetches an object from the store, identified by the key, and copies
// it into an io.Writer.
func (s *SOS) GetTo(key string, wr io.Writer) error {
	if s.base == "" {
		return fmt.Errorf("SOS: Running Get on a destroyed store")
	}

	_, filename := s.getpath(key)
	tmpname := s.tmpfilename()

	// create hard link
	err := os.Link(filename, tmpname)
	if err != nil {
		return fmt.Errorf("SOS: Key does not exist")
	}
	defer os.Remove(tmpname)

	// read value from file
	fh, err := os.Open(tmpname)
	if err != nil {
		return err
	}
	defer fh.Close()

	_, err = io.Copy(wr, fh)
	return err
}

// Delete removes an object from the store.
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
