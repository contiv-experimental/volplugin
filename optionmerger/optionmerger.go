package optionmerger

import (
	"fmt"
	"strings"
)

// Merge the results of a driver options map (coming from a volume create call)
// into a data structure capable of being overlaid on top of a volume parameter
// structure. This allows for nesting support with a `.` in the key.
//
// Note that you *must* use type assertions against both string and
// map[string]interface{} to retrieve the data in this structure.
//
// map[string]string{
//   "a":   "b",
//   "b.c": "d",
//   "b.e": "f",
// }
//
// Turns into
//
// map[string]interface{}{
//   "a": "b",
//   "b": map[string]interface{}{
//     "c": "d",
//     "e": "f",
//   },
// }
func Merge(opts map[string]string) (map[string]interface{}, error) {
	returnMap := map[string]interface{}{}

	for key, value := range opts {
		keyParts := strings.Split(key, ".")
		if len(keyParts) != 1 {
			var innerMap = returnMap
			for i, part := range keyParts {
				if part == "" {
					return nil, fmt.Errorf("Invalid empty subkey")
				}

				if i == len(keyParts)-1 {
					if _, ok := innerMap[part]; ok {
						return nil, fmt.Errorf("Invalid static key sharing namespace with nested key")
					}

					innerMap[part] = value
					break
				}

				if _, ok := innerMap[part]; ok {
					if _, ok := innerMap[part].(map[string]interface{}); !ok {
						return nil, fmt.Errorf("Invalid nested key sharing namespace with static key")
					}

					innerMap = innerMap[part].(map[string]interface{})
				} else {
					innerMap[part] = map[string]interface{}{}
					innerMap = innerMap[part].(map[string]interface{})
				}
			}
		} else {
			if _, ok := returnMap[key]; ok {
				return nil, fmt.Errorf("Invalid static key sharing namespace with nested key")
			}

			returnMap[key] = value
		}
	}

	return returnMap, nil
}
