package command

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var argRegex = regexp.MustCompile(`arg:(\d+)`)

func isArg(tag string) bool {
	return argRegex.MatchString(tag)
}

func getArgValue(tag string, args []string) (*string, error) {
	// Check if the cli tag is an argument
	matches := argRegex.FindStringSubmatch(tag)
	indexStr := matches[1]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		// This should never happen. It means the tag is not formatted correctly.
		return nil, fmt.Errorf("internal failure parsing arguments")
	}
	if len(args) <= index {
		// Assume all args are optional and just return nil for missing args
		return nil, nil
	}

	return &args[index], nil
}

func getStringValue(flags *pflag.FlagSet, args []string, tag string) (*string, error) {
	if isArg(tag) {
		if val, err := getArgValue(tag, args); err != nil {
			return nil, err
		} else {
			return val, nil
		}
	}

	val, err := flags.GetString(tag)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func getIntValue(flags *pflag.FlagSet, args []string, tag string) (*int, error) {
	if isArg(tag) {
		if val, err := getArgValue(tag, args); err != nil {
			return nil, err
		} else {
			if val == nil {
				return nil, nil
			}
			intVal, err := strconv.Atoi(*val)
			if err != nil {
				return nil, fmt.Errorf("invalid value for %s: %s", tag, *val)
			}
			return &intVal, nil
		}
	}

	val, err := flags.GetInt(tag)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func getFloat64Value(flags *pflag.FlagSet, args []string, tag string) (*float64, error) {
	if isArg(tag) {
		if val, err := getArgValue(tag, args); err != nil {
			return nil, err
		} else {
			if val == nil {
				return nil, nil
			}
			floatVal, err := strconv.ParseFloat(*val, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid value for %s: %s", tag, *val)
			}
			return &floatVal, nil
		}
	}

	val, err := flags.GetFloat64(tag)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func getBoolValue(flags *pflag.FlagSet, args []string, tag string) (*bool, error) {
	if isArg(tag) {
		if val, err := getArgValue(tag, args); err != nil {
			return nil, err
		} else {
			if val == nil {
				return nil, nil
			}
			boolVal, err := strconv.ParseBool(*val)
			if err != nil {
				return nil, fmt.Errorf("invalid value for %s: %s", tag, *val)
			}
			return &boolVal, nil
		}
	}

	val, err := flags.GetBool(tag)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func ParseCommand(cmd *cobra.Command, args []string, v any) error {
	flags := cmd.Flags()

	vtype := reflect.TypeOf(v).Elem()
	elem := reflect.ValueOf(v).Elem()

	// Loop through the struct fields
	for i := 0; i < vtype.NumField(); i++ {
		// Get the field
		field := vtype.Field(i)

		// Get the cli tag
		cliTag := field.Tag.Get("cli")
		if cliTag == "" {
			continue
		}

		elemField := elem.FieldByName(field.Name)

		switch field.Type.Kind() {
		case reflect.Ptr:
			switch field.Type.Elem().Kind() {
			case reflect.String:
				val, err := getStringValue(flags, args, cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(val))
			case reflect.Int:
				val, err := getIntValue(flags, args, cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(val))
			case reflect.Float64:
				val, err := getFloat64Value(flags, args, cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(val))
			case reflect.Bool:
				val, err := getBoolValue(flags, args, cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(val))
			}
		case reflect.Slice:
			switch field.Type.Elem().Kind() {
			case reflect.String:
				val, err := flags.GetStringSlice(cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(val))
			case reflect.Int:
				val, err := flags.GetIntSlice(cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(val))
			case reflect.Float64:
				val, err := flags.GetFloat64Slice(cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(val))
			case reflect.Bool:
				val, err := flags.GetBoolSlice(cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(val))
			default:
				return fmt.Errorf("unsupported slice type: %s", field.Type.Elem().Kind())
			}
		case reflect.String:
			val, err := getStringValue(flags, args, cliTag)
			if err != nil {
				return err
			}
			if val != nil {
				elemField.SetString(*val)
			}
		case reflect.Bool:
			val, err := getBoolValue(flags, args, cliTag)
			if err != nil {
				return err
			}
			if val != nil {
				elemField.SetBool(*val)
			}
		case reflect.Int:
			val, err := getIntValue(flags, args, cliTag)
			if err != nil {
				return err
			}
			if val != nil {
				elemField.SetInt(int64(*val))
			}
		case reflect.Float64:
			val, err := getFloat64Value(flags, args, cliTag)
			if err != nil {
				return err
			}
			if val != nil {
				elemField.SetFloat(*val)
			}
		default:
			return fmt.Errorf("unsupported type: %s", field.Type.Kind())
		}
	}

	return nil
}

func InputToString(v any) (string, error) {
	vtype := reflect.TypeOf(v).Elem()
	elem := reflect.ValueOf(v).Elem()

	// Create a slice to store the arguments. The size is the maximum number of fields.
	args := make([]string, vtype.NumField())
	var flagsStr []string

	// Loop through the struct fields
	for i := 0; i < vtype.NumField(); i++ {
		// Get the field
		field := vtype.Field(i)

		// Get the cli tag
		cliTag := field.Tag.Get("cli")
		if cliTag == "" {
			continue
		}

		elemField := elem.FieldByName(field.Name)

		// If the field is a pointer, get the value
		if field.Type.Kind() == reflect.Ptr {
			if elemField.IsNil() {
				continue
			}

			elemField = elemField.Elem()
		}

		var strVal string

		// If the field is a slice, join the values
		if field.Type.Kind() == reflect.Slice {
			if elemField.Len() == 0 {
				continue
			}

			var slice []string
			for i := 0; i < elemField.Len(); i++ {
				slice = append(slice, fmt.Sprintf("%v", elemField.Index(i)))
			}
			strVal = strings.Join(slice, ",")
		} else {
			if elemField.IsZero() {
				continue
			}
			strVal = fmt.Sprintf("%v", elemField)
		}

		if isArg(cliTag) {
			matches := argRegex.FindStringSubmatch(cliTag)
			indexStr := matches[1]
			// This should never error. It means the tag is not formatted correctly.
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return "", fmt.Errorf("internal failure parsing arguments")
			}

			args[index] = strVal
		} else {
			flagsStr = append(flagsStr, fmt.Sprintf("--%s=%s", cliTag, strVal))
		}
	}

	argsString := strings.Trim(strings.Join(args, " "), " ")
	flagsString := strings.Join(flagsStr, " ")

	return strings.Trim(fmt.Sprintf("%s %s", argsString, flagsString), " "), nil
}
