package optionparser

import (
	"reflect"
	"testing"
)

var parsedStructs = map[string]map[string]interface{}{
	"test1": map[string]interface{}{},
	"test2": map[string]interface{}{
		"a": "b",
		"c": "d",
		"e": "f",
	},
	"test3": map[string]interface{}{
		"a": "b",
		"d": map[string]interface{}{
			"e": "f",
			"x": "y",
		},
		"z": "quux",
	},
}

func TestParse(t *testing.T) {
	ret, err := Parse("")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ret, parsedStructs["test1"]) {
		t.Fatal("Empty struct was not equal to empty string output")
	}

	ret, err = Parse("a=b,c=d,e=f")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ret, parsedStructs["test2"]) {
		t.Fatal("basic parameters test failed to parse")
	}

	ret, err = Parse("a=b,d.e=f,d.x=y,z=quux")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ret, parsedStructs["test3"]) {
		t.Fatalf("advanced data structure test failed to parse: %#v", ret)
	}
}

func TestParseNegative(t *testing.T) {
	_, err := Parse("a")
	if err == nil {
		t.Fatal("No key test failed to fail")
	}

	_, err = Parse("a.e")
	if err == nil {
		t.Fatal("Subkey test failed to fail")
	}

	_, err = Parse("a=")
	if err == nil {
		t.Fatal("Empty key test failed to fail")
	}

	_, err = Parse("a.e=")
	if err == nil {
		t.Fatal("Empty subkey test failed to fail")
	}
}
