package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func jsonContent(value any) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func stringArg(args map[string]any, key string) string {
	if value, ok := args[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func stringPtrArg(args map[string]any, key string) *string {
	if value, ok := args[key]; ok && value != nil {
		result := fmt.Sprint(value)
		return &result
	}
	return nil
}

func intArg(args map[string]any, key string) int {
	if value, ok := args[key]; ok && value != nil {
		switch v := value.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case json.Number:
			parsed, _ := strconv.Atoi(v.String())
			return parsed
		case string:
			parsed, _ := strconv.Atoi(v)
			return parsed
		}
	}
	return 0
}

func int64Arg(args map[string]any, key string) int64 {
	return int64(intArg(args, key))
}

func int64SliceArg(args map[string]any, key string) []int64 {
	value, ok := args[key]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]int64, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case int:
			result = append(result, int64(v))
		case int64:
			result = append(result, v)
		case float64:
			result = append(result, int64(v))
		case json.Number:
			parsed, _ := strconv.ParseInt(v.String(), 10, 64)
			if parsed > 0 {
				result = append(result, parsed)
			}
		}
	}
	return result
}

func stringMapArg(args map[string]any, key string) map[string]string {
	value, ok := args[key]
	if !ok || value == nil {
		return map[string]string{}
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	result := map[string]string{}
	for k, v := range raw {
		result[k] = fmt.Sprint(v)
	}
	return result
}
