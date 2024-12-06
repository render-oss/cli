package command

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/ansi"
	"github.com/renderinc/cli/pkg/pointers"
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

func HuhFormFields(cmd *cobra.Command, v any) ([]huh.Field, FormValues) {
	huhFieldMap := make(map[string]huh.Field)
	formValues := FormValuesFromStruct(v)

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

		if flag.Value.Type() == EnumType {
			enumFlag := flag.Value.(*CobraEnum)

			var options []huh.Option[string]
			for _, val := range enumFlag.Values() {
				options = append(options, huh.NewOption[string](val, val))
			}

			if enumFlag.IsMultiSelect() {
				sliceValue := NewStringSliceFormValue(value.String())
				formValues[flag.Name] = sliceValue

				huhFieldMap[flag.Name] = huh.NewMultiSelect[string]().Key(flag.Name).Title(flag.Name).Description(wrappedDescription).Options(options...).Value((*[]string)(sliceValue))
			} else {
				strValue := NewStringFormValue(value.String())
				formValues[flag.Name] = strValue

				huhFieldMap[flag.Name] = huh.NewSelect[string]().Key(flag.Name).Title(flag.Name).Description(wrappedDescription).Options(options...).Value((*string)(strValue))
			}
		} else if flag.Value.Type() == TimeType {
			timeValue := NewStringFormValue(value.String())
			formValues[flag.Name] = timeValue

			huhFieldMap[flag.Name] = huh.NewInput().
				Key(flag.Name).
				Title(flag.Name).
				Description(wrappedDescription).
				Value((*string)(timeValue)).
				Placeholder(fmt.Sprintf("Relative time or %s", time.RFC3339)).
				SuggestionsFunc(func() []string { return TimeSuggestion(timeValue.String()) }, timeValue)
		} else {
			strValue := NewStringFormValue(value.String())
			formValues[flag.Name] = strValue

			huhFieldMap[flag.Name] = huh.NewInput().Key(flag.Name).Title(flag.Name).Description(wrappedDescription).Value((*string)(strValue))
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
