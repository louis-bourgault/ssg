package templating

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/louis-bourgault/ssg/index"
	"github.com/louis-bourgault/ssg/sitepath"
)

var previewPattern = regexp.MustCompile(`^_preview([0-9]+)$`)

func pageProperty(project *index.ProjectIndex, page *Page, property string) (any, error) {
	switch property {
	case "_url":
		return page.OutputURL, nil
	case "_filename":
		return page.Filename, nil
	case "_path":
		if project == nil {
			return nil, fmt.Errorf("project index is unavailable")
		}
		return sitepath.Relative(project.RoutesDir, page.SourcePath)
	}
	if match := previewPattern.FindStringSubmatch(property); match != nil {
		length, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil || length > uint64(^uint(0)>>1) {
			return nil, fmt.Errorf("invalid preview length %q", match[1])
		}
		runes := []rune(page.PlainText)
		if int(length) < len(runes) {
			return string(runes[:int(length)]) + "...", nil
		}
		return page.PlainText, nil
	}
	if strings.HasPrefix(property, "_preview") {
		return nil, fmt.Errorf("invalid preview property %q: length must be a non-negative integer", property)
	}
	if strings.HasPrefix(property, "_") {
		return nil, fmt.Errorf("unknown reserved property %q", property)
	}
	value, exists := page.Meta[property]
	if !exists {
		return nil, fmt.Errorf("page %s has no metadata property %q", page.SourcePath, property)
	}
	return value, nil
}

func headingProperty(heading Heading, property string) (any, error) {
	switch property {
	case "level":
		return heading.Level, nil
	case "text":
		return heading.Text, nil
	case "id":
		return heading.ID, nil
	default:
		return nil, fmt.Errorf("heading has no property %q", property)
	}
}

func scalarString(path string, value any) (string, error) {
	switch value := value.(type) {
	case string:
		return value, nil
	case bool:
		return strconv.FormatBool(value), nil
	case int:
		return strconv.Itoa(value), nil
	case int8:
		return strconv.FormatInt(int64(value), 10), nil
	case int16:
		return strconv.FormatInt(int64(value), 10), nil
	case int32:
		return strconv.FormatInt(int64(value), 10), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case uint:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint64:
		return strconv.FormatUint(value, 10), nil
	case float32:
		return strconv.FormatFloat(float64(value), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(value, 'g', -1, 64), nil
	case json.Number:
		return value.String(), nil
	case time.Time:
		return value.Format(time.RFC3339), nil
	case nil:
		return "", fmt.Errorf("cannot render %s: expected a scalar, got null", path)
	}
	kind := reflect.TypeOf(value).Kind()
	switch kind {
	case reflect.Array, reflect.Slice:
		return "", fmt.Errorf("cannot render %s: expected a scalar, got an array", path)
	case reflect.Map, reflect.Struct:
		return "", fmt.Errorf("cannot render %s: expected a scalar, got an object", path)
	default:
		return "", fmt.Errorf("cannot render %s: expected a scalar, got %s", path, kind)
	}
}

type sortableValue struct {
	kind   string
	text   string
	number *big.Rat
}

func makeSortable(path string, value any) (sortableValue, error) {
	switch value := value.(type) {
	case string:
		return sortableValue{kind: "string", text: value}, nil
	case int:
		return integerSortable(int64(value)), nil
	case int8:
		return integerSortable(int64(value)), nil
	case int16:
		return integerSortable(int64(value)), nil
	case int32:
		return integerSortable(int64(value)), nil
	case int64:
		return integerSortable(value), nil
	case uint:
		return unsignedSortable(uint64(value)), nil
	case uint8:
		return unsignedSortable(uint64(value)), nil
	case uint16:
		return unsignedSortable(uint64(value)), nil
	case uint32:
		return unsignedSortable(uint64(value)), nil
	case uint64:
		return unsignedSortable(value), nil
	case float32:
		return floatSortable(path, float64(value))
	case float64:
		return floatSortable(path, value)
	case json.Number:
		number, ok := new(big.Rat).SetString(value.String())
		if !ok {
			return sortableValue{}, fmt.Errorf("cannot sort %s as a number", path)
		}
		return sortableValue{kind: "number", number: number}, nil
	default:
		return sortableValue{}, fmt.Errorf("cannot sort %s: expected a string or number", path)
	}
}

func integerSortable(value int64) sortableValue {
	return sortableValue{kind: "number", number: new(big.Rat).SetInt64(value)}
}

func unsignedSortable(value uint64) sortableValue {
	integer := new(big.Int).SetUint64(value)
	return sortableValue{kind: "number", number: new(big.Rat).SetInt(integer)}
}

func floatSortable(path string, value float64) (sortableValue, error) {
	number := new(big.Rat).SetFloat64(value)
	if number == nil {
		return sortableValue{}, fmt.Errorf("cannot sort %s: number must be finite", path)
	}
	return sortableValue{kind: "number", number: number}, nil
}
