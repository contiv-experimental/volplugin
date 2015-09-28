package optionmerger

import (
	"reflect"
	"testing"
)

var mergeIO = map[string]map[string]interface{}{
	"test1": {
		"input": map[string]string{
			"a": "b",
			"c": "d",
		},
		"output": map[string]interface{}{
			"a": "b",
			"c": "d",
		},
	},
	"nested": {
		"input": map[string]string{
			"a":   "b",
			"c":   "d",
			"e.f": "g",
			"e.h": "i",
		},
		"output": map[string]interface{}{
			"a": "b",
			"c": "d",
			"e": map[string]interface{}{
				"f": "g",
				"h": "i",
			},
		},
	},
	"double-nested": {
		"input": map[string]string{
			"a":   "b",
			"c":   "d",
			"e.f": "g",
			"e.h": "i",
			"e.j": "k",
			"x.y": "z",
			"x.x": "x",
		},
		"output": map[string]interface{}{
			"a": "b",
			"c": "d",
			"e": map[string]interface{}{
				"f": "g",
				"h": "i",
				"j": "k",
			},
			"x": map[string]interface{}{
				"y": "z",
				"x": "x",
			},
		},
	},
	"deep": {
		"input": map[string]string{
			"a":     "b",
			"c":     "d",
			"e.f.g": "h",
			"e.f.h": "i",
			"e.h":   "i",
			"e.j":   "k",
			"x.y":   "z",
			"x.x":   "x",
		},
		"output": map[string]interface{}{
			"a": "b",
			"c": "d",
			"e": map[string]interface{}{
				"f": map[string]interface{}{
					"g": "h",
					"h": "i",
				},
				"h": "i",
				"j": "k",
			},
			"x": map[string]interface{}{
				"y": "z",
				"x": "x",
			},
		},
	},
}

func TestMerge(t *testing.T) {
	res, err := Merge(mergeIO["test1"]["input"].(map[string]string))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res, mergeIO["test1"]["output"].(map[string]interface{})) {
		t.Fatal("result did not equal expected output")
	}

	res, err = Merge(mergeIO["nested"]["input"].(map[string]string))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res, mergeIO["nested"]["output"].(map[string]interface{})) {
		t.Log(res)
		t.Log(mergeIO["nested"]["output"].(map[string]interface{}))
		t.Fatal("result did not equal expected output")
	}

	res, err = Merge(mergeIO["double-nested"]["input"].(map[string]string))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res, mergeIO["double-nested"]["output"].(map[string]interface{})) {
		t.Log(res)
		t.Log(mergeIO["nested"]["output"].(map[string]interface{}))
		t.Fatal("result did not equal expected output")
	}

	res, err = Merge(mergeIO["deep"]["input"].(map[string]string))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res, mergeIO["deep"]["output"].(map[string]interface{})) {
		t.Log(res)
		t.Log(mergeIO["nested"]["output"].(map[string]interface{}))
		t.Fatal("result did not equal expected output")
	}
}

func TestFailMerge(t *testing.T) {
	res, err := Merge(map[string]string{"a": "a", "a.b": "c"})
	if err == nil {
		t.Log(res)
		t.Fatal("Was able to merge an invalid set of options")
	}

	res, err = Merge(map[string]string{"a.b": "c", "a": "a"})
	if err == nil {
		t.Log(res)
		t.Fatal("Was able to merge an invalid set of options")
	}

	res, err = Merge(map[string]string{"b.b": "c", "b.b.c": "d", "a": "a"})
	if err == nil {
		t.Log(res)
		t.Fatal("Was able to merge an invalid set of options")
	}

	res, err = Merge(map[string]string{"a..b": "c"})
	if err == nil {
		t.Log(res)
		t.Fatal("Was able to merge an invalid set of options")
	}
}
