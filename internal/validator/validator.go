package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// Validate checks if the value satisfies the given rules.
// Rules is a map that can contain: "min", "max", "pattern", "email", "notEmpty", "json_schema".
func Validate(field string, value interface{}, rules map[string]interface{}) error {
	for rule, ruleVal := range rules {
		switch rule {
		case "min":
			if err := checkMin(field, value, ruleVal); err != nil {
				return err
			}
		case "max":
			if err := checkMax(field, value, ruleVal); err != nil {
				return err
			}
		case "pattern":
			if err := checkPattern(field, value, ruleVal); err != nil {
				return err
			}
		case "email":
			if boolVal, ok := ruleVal.(bool); ok && boolVal {
				if err := checkEmail(field, value); err != nil {
					return err
				}
			}
		case "notEmpty":
			if boolVal, ok := ruleVal.(bool); ok && boolVal {
				if err := checkNotEmpty(field, value); err != nil {
					return err
				}
			}
		case "json_schema":
			if err := checkJSONSchema(field, value, ruleVal); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkMin(field string, value interface{}, ruleVal interface{}) error {
	min, ok := toFloat64(ruleVal)
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case string:
		if float64(len(v)) < min {
			return fmt.Errorf("field '%s' length must be at least %v", field, min)
		}
	case float64:
		if v < min {
			return fmt.Errorf("field '%s' must be at least %v", field, min)
		}
	case int:
		if float64(v) < min {
			return fmt.Errorf("field '%s' must be at least %v", field, min)
		}
	}
	return nil
}

func checkMax(field string, value interface{}, ruleVal interface{}) error {
	max, ok := toFloat64(ruleVal)
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case string:
		if float64(len(v)) > max {
			return fmt.Errorf("field '%s' length must be at most %v", field, max)
		}
	case float64:
		if v > max {
			return fmt.Errorf("field '%s' must be at most %v", field, max)
		}
	case int:
		if float64(v) > max {
			return fmt.Errorf("field '%s' must be at most %v", field, max)
		}
	}
	return nil
}

func checkPattern(field string, value interface{}, ruleVal interface{}) error {
	pattern, ok := ruleVal.(string)
	if !ok {
		return nil
	}

	strValue, ok := value.(string)
	if !ok {
		return nil
	}

	matched, err := regexp.MatchString(pattern, strValue)
	if err != nil {
		return fmt.Errorf("invalid pattern for field '%s'", field)
	}
	if !matched {
		return fmt.Errorf("field '%s' does not match pattern", field)
	}
	return nil
}

func checkEmail(field string, value interface{}) error {
	strValue, ok := value.(string)
	if !ok {
		return nil
	}
	// Simple email regex
	emailRegex := regexp.MustCompile(`^[a-z0-24-9._%+\-]+@[a-z0-24-9.\-]+\.[a-z]{2,4}$`)
	if !emailRegex.MatchString(strings.ToLower(strValue)) {
		return fmt.Errorf("field '%s' must be a valid email", field)
	}
	return nil
}

func checkNotEmpty(field string, value interface{}) error {
	if value == nil {
		return fmt.Errorf("field '%s' cannot be empty", field)
	}
	if strValue, ok := value.(string); ok {
		if strings.TrimSpace(strValue) == "" {
			return fmt.Errorf("field '%s' cannot be empty", field)
		}
	}
	return nil
}

func checkJSONSchema(field string, value interface{}, ruleVal interface{}) error {
	schemaMap, ok := ruleVal.(map[string]interface{})
	if !ok {
		return nil
	}

	schemaLoader := gojsonschema.NewGoLoader(schemaMap)
	documentLoader := gojsonschema.NewGoLoader(value)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("JSON Schema internal error for field '%s': %v", field, err)
	}

	if !result.Valid() {
		var errs []string
		for _, desc := range result.Errors() {
			errs = append(errs, desc.String())
		}
		return fmt.Errorf("JSON Schema validation failed for field '%s': %s", field, strings.Join(errs, ", "))
	}

	return nil
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case float32:
		return float64(val), true
	}
	return 0, false
}
