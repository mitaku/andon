//go:build unix

package main

import (
	"reflect"
	"testing"

	"andon/internal/dispatcher"
)

// The copy in dispatcherstate.go has the dispatcher's State's JSON keys and value kinds, all the way down.
func TestDispatcherStateShape(t *testing.T) {
	var same func(path string, a, b reflect.Type)
	same = func(path string, a, b reflect.Type) {
		if a.Kind() != b.Kind() {
			t.Errorf("%s: %s in internal/dispatcher, %s in kanban", path, a.Kind(), b.Kind())
			return
		}
		switch a.Kind() {
		case reflect.Pointer, reflect.Slice:
			same(path+"[]", a.Elem(), b.Elem())
		case reflect.Map:
			same(path+"{key}", a.Key(), b.Key())
			same(path+"{}", a.Elem(), b.Elem())
		case reflect.Struct:
			if a.NumField() != b.NumField() {
				t.Errorf("%s: %d fields in internal/dispatcher, %d in kanban", path, a.NumField(), b.NumField())
				return
			}
			for i := 0; i < a.NumField(); i++ {
				fa, fb := a.Field(i), b.Field(i)
				if fa.Tag.Get("json") != fb.Tag.Get("json") {
					t.Errorf("%s.%s: json %q in internal/dispatcher, %q in kanban", path, fa.Name, fa.Tag.Get("json"), fb.Tag.Get("json"))
					continue
				}
				same(path+"."+fa.Name, fa.Type, fb.Type)
			}
		}
	}
	same("State", reflect.TypeOf(dispatcher.State{}), reflect.TypeOf(dispatcherState{}))
}
