package optionparser

import (
	"fmt"
	"strings"
)

// Parse a set of options in this format:
//
// key=value,key2.subkey=subvalue,key3.subkey.subkey2=value
// The return value is map[string]interface{}, and is laid out in
// string+map[string]string throughout the struct.
//
// Therefore, it is necessary to type assert all values returned by this
// parser.
//
// FIXME if would be nice if this used reflection instead of the current
// situation, but that implementation could take a week by itself.
//
func Parse(str string) (map[string]interface{}, error) {
	returnMap := map[string]interface{}{}

	if str == "" {
		return returnMap, nil
	}

	parts := strings.Split(str, ",")
	for _, part := range parts {
		inner := strings.SplitN(part, "=", 2)
		if len(inner) != 2 {
			return nil, fmt.Errorf("Invalid syntax: %q", part)
		}

		key := inner[0]
		value := inner[1]

		if value == "" {
			return nil, fmt.Errorf("Invalid syntax: %q", part)
		}

		keyParts := strings.Split(key, ".")

		var innerMap map[string]interface{}

		if len(keyParts) != 1 {
			innerMap = returnMap
			for i := 0; i < len(keyParts); i++ {
				if len(keyParts)-1 == i {
					innerMap[keyParts[i]] = value
					break
				}

				switch innerMap[keyParts[i]].(type) {
				case map[string]interface{}:
					tmp, ok := innerMap[keyParts[i]].(map[string]interface{})
					if ok {
						innerMap = tmp
						break
					}
					innerMap[keyParts[i]] = map[string]interface{}{}
					innerMap = innerMap[keyParts[i]].(map[string]interface{})
				default:
					innerMap[keyParts[i]] = map[string]interface{}{}
					innerMap = innerMap[keyParts[i]].(map[string]interface{})
				}
			}
		} else {
			returnMap[key] = value
		}
	}
	return returnMap, nil
}
