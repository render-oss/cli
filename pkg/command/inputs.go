package command

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

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

func getTimeValue(flags *pflag.FlagSet, tag string) (*TimeOrRelative, error) {
	if flag := flags.Lookup(tag); flag != nil {
		if flag.Value.Type() == TimeType {
			cobraTime, ok := flag.Value.(*CobraTime)
			if !ok {
				return nil, fmt.Errorf("unexpected time type")
			}
			val := cobraTime.t
			return val, nil
		}
	}

	return nil, nil
}

func getStringValue(flags *pflag.FlagSet, args []string, tag string) (*string, error) {
	if isArg(tag) {
		if val, err := getArgValue(tag, args); err != nil {
			return nil, err
		} else {
			return val, nil
		}
	}

	flag := flags.Lookup(tag)

	if flag == nil {
		return nil, nil
	}

	if flag.Value.Type() == EnumType {
		val := flag.Value.String()
		return &val, nil
	}

	val, err := flags.GetString(tag)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func getStringSliceValue(flags *pflag.FlagSet, tag string) ([]string, error) {
	if flag := flags.Lookup(tag); flag != nil {
		if flag.Value.Type() == EnumType {
			cobraEnum, ok := flag.Value.(*CobraEnum)
			if !ok {
				return nil, fmt.Errorf("unexpected enum type")
			}

			val := cobraEnum.SelectedValues()
			return val, nil
		}

		// StringArray flags support multiple invocations: --flag a --flag b
		// Unlike StringSlice which expects comma-separated: --flag=a,b
		if flag.Value.Type() == "stringArray" {
			val, err := flags.GetStringArray(tag)
			if err != nil {
				return nil, err
			}
			return val, nil
		}
	}

	val, err := flags.GetStringSlice(tag)
	if err != nil {
		return nil, err
	}
	return val, nil
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

func shouldSetPointerField(flags *pflag.FlagSet, cliTag string) bool {
	return isArg(cliTag) || flags.Changed(cliTag)
}

func setStringPointerField(elemField reflect.Value, shouldSet bool, val *string) {
	if !shouldSet || val == nil {
		elemField.SetZero()
		return
	}
	ptr := reflect.New(elemField.Type().Elem())
	ptr.Elem().SetString(*val)
	elemField.Set(ptr)
}

func setIntPointerField(elemField reflect.Value, shouldSet bool, val *int) {
	if !shouldSet || val == nil {
		elemField.SetZero()
		return
	}
	ptr := reflect.New(elemField.Type().Elem())
	ptr.Elem().SetInt(int64(*val))
	elemField.Set(ptr)
}

func setFloat64PointerField(elemField reflect.Value, shouldSet bool, val *float64) {
	if !shouldSet || val == nil {
		elemField.SetZero()
		return
	}
	ptr := reflect.New(elemField.Type().Elem())
	ptr.Elem().SetFloat(*val)
	elemField.Set(ptr)
}

func setBoolPointerField(elemField reflect.Value, shouldSet bool, val *bool) {
	if !shouldSet || val == nil {
		elemField.SetZero()
		return
	}
	ptr := reflect.New(elemField.Type().Elem())
	ptr.Elem().SetBool(*val)
	elemField.Set(ptr)
}

func ParseCommandInteractiveOnly(cmd *cobra.Command, args []string, v any) error {
	format := GetFormatFromContext(cmd.Context())
	if !format.Interactive() {
		return fmt.Errorf("`%s` can only be used in interactive mode", cmd.CommandPath())
	}

	return ParseCommand(cmd, args, v)
}

type Validator interface {
	Validate(interactive bool) error
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
			if field.Type == reflect.TypeOf(&TimeOrRelative{}) {
				val, err := getTimeValue(flags, cliTag)
				if err != nil {
					return err
				}

				elemField.Set(reflect.ValueOf(val))
				continue
			}

			switch field.Type.Elem().Kind() {
			case reflect.String:
				val, err := getStringValue(flags, args, cliTag)
				if err != nil {
					return err
				}
				setStringPointerField(elemField, shouldSetPointerField(flags, cliTag), val)
			case reflect.Int:
				val, err := getIntValue(flags, args, cliTag)
				if err != nil {
					return err
				}
				setIntPointerField(elemField, shouldSetPointerField(flags, cliTag), val)
			case reflect.Float64:
				val, err := getFloat64Value(flags, args, cliTag)
				if err != nil {
					return err
				}
				setFloat64PointerField(elemField, shouldSetPointerField(flags, cliTag), val)
			case reflect.Bool:
				val, err := getBoolValue(flags, args, cliTag)
				if err != nil {
					return err
				}
				setBoolPointerField(elemField, shouldSetPointerField(flags, cliTag), val)
			}
		case reflect.Slice:
			switch field.Type.Elem().Kind() {
			case reflect.String:
				val, err := getStringSliceValue(flags, cliTag)
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

	if val, ok := v.(Validator); ok {
		if err := val.Validate(IsInteractive(cmd.Context())); err != nil {
			return err
		}
	}

	if !IsInteractive(cmd.Context()) {
		if err := ValidateRequiredFields(v); err != nil {
			return err
		}
	}

	return nil
}

// InputToString formats the cli-tagged fields of v as shell-quoted command
// arguments. []string fields are encoded with pflag's StringSlice CSV grammar;
// use CommandName to honor the effective flag types of a specific command.
func InputToString(v any) (string, error) {
	return inputToString(v, nil)
}

func inputToString(v any, flags *pflag.FlagSet) (string, error) {
	args, err := inputArgs(v, flags)
	if err != nil {
		return "", err
	}
	return strings.Join(args, " "), nil
}

// inputArgs converts the cli-tagged fields of v into shell-quoted argv tokens:
// positional arguments in index order, followed by flags. When flags is
// non-nil, slice fields are encoded for the registered flag type so that
// parsing the tokens again with ParseCommand yields the same values.
func inputArgs(v any, flags *pflag.FlagSet) ([]string, error) {
	vtype := reflect.TypeOf(v).Elem()
	elem := reflect.ValueOf(v).Elem()

	// Positional arguments by index. The size is the maximum number of fields.
	args := make([]string, vtype.NumField())
	var flagArgs []string

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

		// Time flags keep the relative text the user entered or the full
		// absolute timestamp, in a form accepted by CobraTime.Set.
		if field.Type == reflect.TypeOf(&TimeOrRelative{}) {
			strVal, ok := timeArgValue(elemField.Interface().(*TimeOrRelative))
			if !ok {
				continue
			}
			flagArgs = append(flagArgs, flagArg(cliTag, strVal))
			continue
		}

		// If the field is a pointer, get the value
		if field.Type.Kind() == reflect.Ptr {
			if elemField.IsNil() {
				continue
			}

			elemField = elemField.Elem()
		}

		// Slices are encoded according to the effective flag type
		if elemField.Kind() == reflect.Slice {
			if elemField.Len() == 0 {
				continue
			}

			var slice []string
			for i := 0; i < elemField.Len(); i++ {
				slice = append(slice, fmt.Sprintf("%v", elemField.Index(i)))
			}

			var flag *pflag.Flag
			if flags != nil {
				flag = flags.Lookup(cliTag)
			}

			switch {
			case flag != nil && flag.Value.Type() == "stringArray":
				// StringArray flags take one element per invocation
				for _, s := range slice {
					flagArgs = append(flagArgs, flagArg(cliTag, s))
				}
			case elemField.Type().Elem().Kind() == reflect.String && (flag == nil || flag.Value.Type() == "stringSlice"):
				strVal, err := stringSliceArgValue(slice)
				if err != nil {
					return nil, err
				}
				flagArgs = append(flagArgs, flagArg(cliTag, strVal))
			default:
				// Enum and numeric/bool slice flags split on bare commas
				flagArgs = append(flagArgs, flagArg(cliTag, strings.Join(slice, ",")))
			}
			continue
		}

		if elemField.IsZero() {
			continue
		}
		strVal := fmt.Sprintf("%v", elemField)

		if isArg(cliTag) {
			matches := argRegex.FindStringSubmatch(cliTag)
			indexStr := matches[1]
			// This should never error. It means the tag is not formatted correctly.
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("internal failure parsing arguments")
			}

			args[index] = shellQuote(strVal)
		} else {
			flagArgs = append(flagArgs, flagArg(cliTag, strVal))
		}
	}

	var result []string
	for _, arg := range args {
		if arg != "" {
			result = append(result, arg)
		}
	}
	return append(result, flagArgs...), nil
}

func timeArgValue(t *TimeOrRelative) (string, bool) {
	switch {
	case t == nil:
		return "", false
	case t.Relative != nil:
		return *t.Relative, true
	case t.T != nil:
		return t.T.Format(time.RFC3339Nano), true
	default:
		return "", false
	}
}

// stringSliceArgValue encodes values with the CSV grammar pflag's StringSlice
// flags parse, so commas, quotes and spaces inside an element are retained.
func stringSliceArgValue(values []string) (string, error) {
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)
	if err := w.Write(values); err != nil {
		return "", err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func flagArg(name, value string) string {
	return "--" + name + "=" + shellQuote(value)
}

var shellSafeRegex = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote quotes s for POSIX shells when it contains characters the shell
// would otherwise interpret.
func shellQuote(s string) string {
	if shellSafeRegex.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
