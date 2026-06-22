package command

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/ansi"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type FormValue interface {
	String() string
}

type stringFormValue string

func (s *stringFormValue) String() string {
	return (string)(*s)
}

func NewStringFormValue(s string) *stringFormValue {
	return (*stringFormValue)(&s)
}

type stringSliceFormValue []string

func NewStringSliceFormValue(s string) *stringSliceFormValue {
	slice := strings.Split(s, ",")
	return (*stringSliceFormValue)(&slice)
}

func (s *stringSliceFormValue) String() string {
	str := strings.Join(*s, ",")
	return str
}

type FormValues map[string]FormValue

func FormValuesFromStruct(v any) FormValues {
	if reflect.TypeOf(v).Kind() != reflect.Ptr {
		panic("v must be a pointer")
	}

	formValues := make(FormValues)
	vtype := reflect.TypeOf(v).Elem()
	elem := reflect.ValueOf(v).Elem()

	for i := 0; i < vtype.NumField(); i++ {
		// Get the field
		field := vtype.Field(i)

		// Get the cli tag
		cliTag := field.Tag.Get("cli")

		elemField := elem.FieldByName(field.Name)

		switch field.Type.Kind() {
		case reflect.Ptr:
			if elemField.IsNil() {
				formValues[cliTag] = NewStringFormValue("")
				continue
			}

			if field.Type == reflect.TypeOf(&TimeOrRelative{}) {
				val := elemField.Interface().(*TimeOrRelative)
				formValues[cliTag] = NewStringFormValue(val.String())
				continue
			}

			switch field.Type.Elem().Kind() {
			case reflect.String:
				val := elemField.Interface().(*string)
				formValues[cliTag] = NewStringFormValue(*val)
			case reflect.Int:
				val := elemField.Interface().(*int)
				formValues[cliTag] = NewStringFormValue(fmt.Sprintf("%d", *val))
			case reflect.Float64:
				val := elemField.Interface().(*float64)
				formValues[cliTag] = NewStringFormValue(fmt.Sprintf("%f", *val))
			case reflect.Bool:
				val := elemField.Interface().(*bool)
				formValues[cliTag] = NewStringFormValue(fmt.Sprintf("%t", *val))
			}
		case reflect.Slice:
			switch field.Type.Elem().Kind() {
			case reflect.String:
				val := elemField.Interface().([]string)
				formValues[cliTag] = NewStringFormValue(strings.Join(val, ","))
			case reflect.Int:
				val := elemField.Interface().([]int)
				var strs []string
				for _, v := range val {
					strs = append(strs, fmt.Sprintf("%d", v))
				}
				formValues[cliTag] = NewStringFormValue(strings.Join(strs, ","))
			case reflect.Float64:
				val := elemField.Interface().([]float64)
				var strs []string
				for _, v := range val {
					strs = append(strs, fmt.Sprintf("%f", v))
				}
				formValues[cliTag] = NewStringFormValue(strings.Join(strs, ","))
			case reflect.Bool:
				val := elemField.Interface().([]bool)
				var strs []string
				for _, v := range val {
					strs = append(strs, fmt.Sprintf("%t", v))
				}
				formValues[cliTag] = NewStringFormValue(strings.Join(strs, ","))
			default:
				panic(fmt.Sprintf("unsupported slice type: %s", field.Type.Elem().Kind()))
			}
		case reflect.String:
			val := elemField.Interface().(string)
			formValues[cliTag] = NewStringFormValue(val)
		case reflect.Bool:
			val := elemField.Interface().(bool)
			formValues[cliTag] = NewStringFormValue(fmt.Sprintf("%t", val))
		case reflect.Int:
			val := elemField.Interface().(int)
			formValues[cliTag] = NewStringFormValue(fmt.Sprintf("%d", val))
		case reflect.Float64:
			val := elemField.Interface().(float64)
			formValues[cliTag] = NewStringFormValue(fmt.Sprintf("%f", val))
		case reflect.Struct:
			// skip nested structs
			continue
		default:
			panic(fmt.Sprintf("unsupported type: %s", field.Type.Kind()))
		}
	}

	return formValues
}

func arrayFromString(str string) []string {
	if str == "" {
		return []string{}
	}

	return strings.Split(str, ",")
}

func StructFromFormValues(formValues FormValues, v any) error {
	if reflect.TypeOf(v).Kind() != reflect.Ptr {
		return fmt.Errorf("v must be a pointer")
	}

	vtype := reflect.TypeOf(v).Elem()
	elem := reflect.ValueOf(v).Elem()

	for i := 0; i < vtype.NumField(); i++ {
		// Get the field
		field := vtype.Field(i)

		// Get the cli tag
		cliTag := field.Tag.Get("cli")

		elemField := elem.FieldByName(field.Name)

		switch field.Type.Kind() {
		case reflect.Ptr:
			if field.Type == reflect.TypeOf(&TimeOrRelative{}) {
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}

				timeOrRelative, err := ParseTime(time.Now(), pointers.From(val.String()))
				if err != nil {
					return err
				}

				elemField.Set(reflect.ValueOf(timeOrRelative))
				continue
			}

			switch field.Type.Elem().Kind() {
			case reflect.String:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				elemField.Set(reflect.ValueOf(pointers.From(val.String())))
			case reflect.Int:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				intVal, err := strconv.Atoi(val.String())
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(intVal))
			case reflect.Float64:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				floatVal, err := strconv.ParseFloat(val.String(), 64)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(floatVal))
			case reflect.Bool:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				boolVal, err := strconv.ParseBool(val.String())
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(boolVal))
			default:
				return fmt.Errorf("unsupported pointer type: %s", field.Type.Elem().Kind())
			}
		case reflect.Slice:
			switch field.Type.Elem().Kind() {
			case reflect.String:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				elemField.Set(reflect.ValueOf(arrayFromString(val.String())))
			case reflect.Int:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				var intVals []int
				for _, v := range arrayFromString(val.String()) {
					intVal, err := strconv.Atoi(v)
					if err != nil {
						return err
					}
					intVals = append(intVals, intVal)
				}
				elemField.Set(reflect.ValueOf(intVals))
			case reflect.Float64:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				var floatVals []float64
				for _, v := range arrayFromString(val.String()) {
					floatVal, err := strconv.ParseFloat(v, 64)
					if err != nil {
						return err
					}
					floatVals = append(floatVals, floatVal)
				}
				elemField.Set(reflect.ValueOf(floatVals))
			case reflect.Bool:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				var boolVals []bool
				for _, v := range arrayFromString(val.String()) {
					boolVal, err := strconv.ParseBool(v)
					if err != nil {
						return err
					}
					boolVals = append(boolVals, boolVal)
				}
				elemField.Set(reflect.ValueOf(boolVals))
			default:
				return fmt.Errorf("unsupported slice type: %s", field.Type.Elem().Kind())
			}
		case reflect.String:
			val, ok := formValues[cliTag]
			if !ok {
				continue
			}
			elemField.SetString(val.String())
		case reflect.Bool:
			val, ok := formValues[cliTag]
			if !ok {
				continue
			}
			boolVal, err := strconv.ParseBool(val.String())
			if err != nil {
				return err
			}
			elemField.SetBool(boolVal)
		case reflect.Int:
			val, ok := formValues[cliTag]
			if !ok {
				continue
			}
			intVal, err := strconv.Atoi(val.String())
			if err != nil {
				return err
			}
			elemField.SetInt(int64(intVal))
		case reflect.Float64:
			val, ok := formValues[cliTag]
			if !ok {
				continue
			}
			floatVal, err := strconv.ParseFloat(val.String(), 64)
			if err != nil {
				return err
			}
			elemField.SetFloat(floatVal)
		case reflect.Struct:
			// skip nested structs
		default:
			return fmt.Errorf("unsupported type: %s", field.Type.Kind())
		}
	}
	return nil
}

// preferredEditor returns $EDITOR if set, otherwise vi. vi is the fallback
// because nano's "File Name to Write:" prompt on save is unexpected for users
// who just want to edit and confirm.
func preferredEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return "vi"
}

type textareaConfig struct {
	lines int
	ext   string
}

func chainStringValidators(validators ...func(string) error) func(string) error {
	return func(s string) error {
		for _, validate := range validators {
			if err := validate(s); err != nil {
				return err
			}
		}
		return nil
	}
}

func requiredFieldError(name string, cliFlag bool) error {
	prefix := ""
	if cliFlag {
		prefix = "--"
	}
	return fmt.Errorf("%s%s is required", prefix, name)
}

func requiredFieldStringValidator(name string) func(string) error {
	return func(s string) error {
		if s == "" {
			return requiredFieldError(name, false)
		}
		return nil
	}
}

func requiredFieldSliceValidator(name string) func([]string) error {
	return func(s []string) error {
		if len(s) == 0 {
			return requiredFieldError(name, false)
		}
		return nil
	}
}

// requiredFieldCLITags returns cli flag names for fields tagged validate:"required".
func requiredFieldCLITags(v any) map[string]bool {
	required := make(map[string]bool)
	vtype := reflect.TypeOf(v).Elem()
	for field := range vtype.Fields() {
		if field.Tag.Get("validate") != "required" {
			continue
		}
		if cliTag := field.Tag.Get("cli"); cliTag != "" {
			required[cliTag] = true
		}
	}
	return required
}

// ValidateRequiredFields returns an error when a field tagged validate:"required"
// is empty. Used for non-interactive flag parsing.
func ValidateRequiredFields(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("value must be a non-nil pointer")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("value must be a pointer to struct")
	}

	rt := rv.Type()
	for field := range rt.Fields() {
		if field.Tag.Get("validate") != "required" {
			continue
		}
		cliTag := field.Tag.Get("cli")
		if cliTag == "" {
			continue
		}
		if isEmptyValue(rv.FieldByName(field.Name)) {
			return requiredFieldError(cliTag, true)
		}
	}
	return nil
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return true
		}
		return isEmptyValue(v.Elem())
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Array:
		return v.Len() == 0
	default:
		return v.IsZero()
	}
}

// textareaConfigFromStruct reads cli-lines and cli-ext struct tags in a single
// pass and returns a map from cli flag name to textarea config.
func textareaConfigFromStruct(v any) map[string]textareaConfig {
	configs := make(map[string]textareaConfig)
	vtype := reflect.TypeOf(v).Elem()
	for i := 0; i < vtype.NumField(); i++ {
		field := vtype.Field(i)
		cliTag := field.Tag.Get("cli")
		linesStr := field.Tag.Get("cli-lines")
		ext := field.Tag.Get("cli-ext")
		if linesStr == "" && ext == "" {
			continue
		}
		cfg := configs[cliTag]
		if linesStr != "" {
			if n, err := strconv.Atoi(linesStr); err == nil {
				cfg.lines = n
			}
		}
		if ext != "" {
			cfg.ext = ext
		}
		configs[cliTag] = cfg
	}
	return configs
}

func HuhFormFields(cmd *cobra.Command, v any) ([]huh.Field, FormValues) {
	huhFieldMap := make(map[string]huh.Field)
	formValues := FormValuesFromStruct(v)
	textareaConfigs := textareaConfigFromStruct(v)
	requiredFields := requiredFieldCLITags(v)

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		// If the flag is not in the form values, skip it
		if _, ok := formValues[flag.Name]; !ok {
			return
		}

		value := formValues[flag.Name]

		if value == nil {
			value = NewStringFormValue(flag.DefValue)
		}

		// We have to wrap the description because of this bug in lipgloss: https://github.com/charmbracelet/lipgloss/issues/85
		// It's unfortunate to set a default width of 53, but this should work with our current
		// filter component. We can adjust if needed.
		wrappedDescription := ansi.Wrap(flag.Usage, 53, "-")

		title := flag.Name
		if requiredFields[flag.Name] {
			title += " *"
		}

		if flag.Value.Type() == EnumType {
			enumFlag := flag.Value.(*CobraEnum)

			var options []huh.Option[string]
			for _, val := range enumFlag.Values() {
				options = append(options, huh.NewOption[string](val, val))
			}

			if enumFlag.IsMultiSelect() {
				sliceValue := NewStringSliceFormValue(value.String())
				formValues[flag.Name] = sliceValue

				field := huh.NewMultiSelect[string]().Key(flag.Name).Title(title).Description(wrappedDescription).Options(options...).Value((*[]string)(sliceValue))
				if requiredFields[flag.Name] {
					field = field.Validate(requiredFieldSliceValidator(flag.Name))
				}
				huhFieldMap[flag.Name] = field
			} else {
				strValue := NewStringFormValue(value.String())
				formValues[flag.Name] = strValue

				field := huh.NewSelect[string]().Key(flag.Name).Title(title).Description(wrappedDescription).Options(options...).Value((*string)(strValue))
				if requiredFields[flag.Name] {
					field = field.Validate(requiredFieldStringValidator(flag.Name))
				}
				huhFieldMap[flag.Name] = field
			}
		} else if flag.Value.Type() == TimeType {
			timeValue := NewStringFormValue(value.String())
			formValues[flag.Name] = timeValue

			field := huh.NewInput().
				Key(flag.Name).
				Title(title).
				Description(wrappedDescription).
				Value((*string)(timeValue)).
				Placeholder(fmt.Sprintf("Relative time or %s", time.RFC3339)).
				SuggestionsFunc(func() []string { return TimeSuggestion(timeValue.String()) }, timeValue)
			if requiredFields[flag.Name] {
				field = field.Validate(requiredFieldStringValidator(flag.Name))
			}
			huhFieldMap[flag.Name] = field
		} else if cfg, ok := textareaConfigs[flag.Name]; ok && cfg.lines > 0 {
			strValue := NewStringFormValue(value.String())
			formValues[flag.Name] = strValue

			field := huh.NewText().Key(flag.Name).Title(title).Description(wrappedDescription).Value((*string)(strValue)).Lines(cfg.lines).CharLimit(0)
			var validators []func(string) error
			if requiredFields[flag.Name] {
				validators = append(validators, requiredFieldStringValidator(flag.Name))
			}
			editor := preferredEditor()
			if cfg.ext != "" {
				if cfg.ext == "json" {
					validators = append(validators, func(s string) error {
						if s != "" && !json.Valid([]byte(s)) {
							return fmt.Errorf("input must be valid JSON")
						}
						return nil
					})
				}
				field = field.Editor(editor).EditorExtension(cfg.ext)
			} else {
				field = field.Editor(editor)
			}
			if len(validators) > 0 {
				field = field.Validate(chainStringValidators(validators...))
			}
			huhFieldMap[flag.Name] = field
		} else {
			strValue := NewStringFormValue(value.String())
			formValues[flag.Name] = strValue

			field := huh.NewInput().Key(flag.Name).Title(title).Description(wrappedDescription).Value((*string)(strValue))
			if requiredFields[flag.Name] {
				field = field.Validate(requiredFieldStringValidator(flag.Name))
			}
			huhFieldMap[flag.Name] = field
		}
	})

	// Order the fields in the form by the order they have in the struct
	var fields []huh.Field
	vtype := reflect.TypeOf(v).Elem()
	for i := 0; i < vtype.NumField(); i++ {
		// Get the field
		field := vtype.Field(i)

		// Get the cli tag
		cliTag := field.Tag.Get("cli")

		if huhField, ok := huhFieldMap[cliTag]; ok {
			fields = append(fields, huhField)
		}
	}

	return fields, formValues
}
