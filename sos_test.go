// (c) 2020 by Harald Weidner
//
// This library is released under the GNU Lesser General Public License
// version 3 (LGPLv3). See the LICENSE file for details.

package sos

import "testing"

func TestSOS(t *testing.T) {
	s, _ := New("./._sostest")

	key := "hello"
	obj1 := ([]byte)("world")

	s.Store(key, obj1)

	obj2, _ := s.Get(key)
	if string(obj1) != string(obj2) {
		t.Errorf("Got %v from store, expected %v", obj2, obj1)
	}

	s.Delete(key)
	obj3, _ := s.Get(key)
	if string(obj3) != "" {
		t.Errorf("Got non empty value for deleted object")
	}

	s.Destroy()
	s.Store(key, obj1)
	obj4, _ := s.Get(key)
	if string(obj4) != "" {
		t.Errorf("Got non empty value from destroyed object store")
	}
}
