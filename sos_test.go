// (c) 2020-2021 by Harald Weidner
//
// This library is released under the Mozilla License, version 2 (MPLv2).
// See the LICENSE file for details.

package sos

import (
	"bytes"
	"testing"
)

// Test the byte slice and string interface
func TestSOS(t *testing.T) {
	s, _ := New("./._sostest")

	key1, val1 := "hello", "world"
	key2, val2 := "foo", "bar"

	s.StoreString(key1, val1)
	s.Store(key2, []byte(val2))

	obj1s, _ := s.Get(key1)
	obj1 := string(obj1s)
	obj2, _ := s.GetString(key2)

	if obj1 != val1 {
		t.Errorf("Got %s from store, expected %s", obj1, val1)
	}

	if obj2 != val2 {
		t.Errorf("Got %s from store, expected %s", obj2, val2)
	}

	s.Delete(key1)
	obj3, _ := s.Get(key1)
	if len(obj3) > 0 {
		t.Errorf("Got non empty value for deleted object")
	}

	s.Destroy()
	s.StoreString(key1, val1)
	obj4, _ := s.Get(key1)
	if len(obj4) > 0 {
		t.Errorf("Got non empty value from destroyed object store")
	}
}

// Test the Reader/Writer interface directly
func TestSOSFile(t *testing.T) {
	s, _ := New("./._sostest")

	key := "hello"
	s1 := "world"
	obj1 := bytes.NewBufferString(s1)

	s.StoreFrom(key, obj1)

	obj2 := new(bytes.Buffer)
	_ = s.GetTo(key, obj2)
	s2, _ := obj2.ReadString(':')
	if s2 != s1 {
		t.Errorf("Got %v from store, expected %v", s2, s1)
	}

	s.Delete(key)
	s.Destroy()
}
