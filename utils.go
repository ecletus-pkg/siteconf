package siteconf

import (
	"reflect"

	path_helpers "github.com/moisespsena-go/path-helpers"
)

func PrivateConfName(v interface{}) string {
	t := reflect.ValueOf(v).Type()
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	name := path_helpers.PkgPathOf(t) + "." + t.Name()
	return name
}
