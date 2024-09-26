package command

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type FormValues map[string]*string

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
				formValues[cliTag] = pointers.From("")
			}

			switch field.Type.Elem().Kind() {
			case reflect.String:
				val := elemField.Interface().(*string)
				formValues[cliTag] = val
			case reflect.Int:
				val := elemField.Interface().(*int)
				formValues[cliTag] = pointers.From(fmt.Sprintf("%d", *val))
			case reflect.Float64:
				val := elemField.Interface().(*float64)
				formValues[cliTag] = pointers.From(fmt.Sprintf("%f", *val))
			case reflect.Bool:
				val := elemField.Interface().(*bool)
				formValues[cliTag] = pointers.From(fmt.Sprintf("%t", *val))
			}
		case reflect.Slice:
			switch field.Type.Elem().Kind() {
			case reflect.String:
				val := elemField.Interface().([]string)
				formValues[cliTag] = pointers.From(strings.Join(val, ","))
			case reflect.Int:
				val := elemField.Interface().([]int)
				var strs []string
				for _, v := range val {
					strs = append(strs, fmt.Sprintf("%d", v))
				}
				formValues[cliTag] = pointers.From(strings.Join(strs, ","))
			case reflect.Float64:
				val := elemField.Interface().([]float64)
				var strs []string
				for _, v := range val {
					strs = append(strs, fmt.Sprintf("%f", v))
				}
				formValues[cliTag] = pointers.From(strings.Join(strs, ","))
			case reflect.Bool:
				val := elemField.Interface().([]bool)
				var strs []string
				for _, v := range val {
					strs = append(strs, fmt.Sprintf("%t", v))
				}
				formValues[cliTag] = pointers.From(strings.Join(strs, ","))
			default:
				panic(fmt.Sprintf("unsupported slice type: %s", field.Type.Elem().Kind()))
			}
		case reflect.String:
			val := elemField.Interface().(string)
			formValues[cliTag] = &val
		case reflect.Bool:
			val := elemField.Interface().(bool)
			formValues[cliTag] = pointers.From(fmt.Sprintf("%t", val))
		case reflect.Int:
			val := elemField.Interface().(int)
			formValues[cliTag] = pointers.From(fmt.Sprintf("%d", val))
		case reflect.Float64:
			val := elemField.Interface().(float64)
			formValues[cliTag] = pointers.From(fmt.Sprintf("%f", val))
		default:
			panic(fmt.Sprintf("unsupported type: %s", field.Type.Kind()))
		}
	}

	return formValues
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
			switch field.Type.Elem().Kind() {
			case reflect.String:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				elemField.Set(reflect.ValueOf(val))
			case reflect.Int:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				intVal, err := strconv.Atoi(*val)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(intVal))
			case reflect.Float64:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				floatVal, err := strconv.ParseFloat(*val, 64)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(floatVal))
			case reflect.Bool:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				boolVal, err := strconv.ParseBool(*val)
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
				elemField.Set(reflect.ValueOf(strings.Split(*val, ",")))
			case reflect.Int:
				val, ok := formValues[cliTag]
				if !ok {
					continue
				}
				var intVals []int
				for _, v := range strings.Split(*val, ",") {
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
				for _, v := range strings.Split(*val, ",") {
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
				for _, v := range strings.Split(*val, ",") {
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
			elemField.SetString(*val)
		case reflect.Bool:
			val, ok := formValues[cliTag]
			if !ok {
				continue
			}
			boolVal, err := strconv.ParseBool(*val)
			if err != nil {
				return err
			}
			elemField.SetBool(boolVal)
		case reflect.Int:
			val, ok := formValues[cliTag]
			if !ok {
				continue
			}
			intVal, err := strconv.Atoi(*val)
			if err != nil {
				return err
			}
			elemField.SetInt(int64(intVal))
		case reflect.Float64:
			val, ok := formValues[cliTag]
			if !ok {
				continue
			}
			floatVal, err := strconv.ParseFloat(*val, 64)
			if err != nil {
				return err
			}
			elemField.SetFloat(floatVal)
		default:
			return fmt.Errorf("unsupported type: %s", field.Type.Kind())
		}
	}
	return nil
}

func HuhForm(cmd *cobra.Command, v any) (*huh.Form, FormValues) {
	huhFieldMap := make(map[string]huh.Field)
	formValues := FormValuesFromStruct(v)

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		// If the flag is not in the form values, skip it
		if _, ok := formValues[flag.Name]; !ok {
			return
		}

		value := formValues[flag.Name]
		if value == nil {
			value = pointers.From(flag.DefValue)
		}

		huhFieldMap[flag.Name] = huh.NewInput().Key(flag.Name).Title(flag.Name).Description(flag.Usage).Placeholder(flag.DefValue).Value(value)
		formValues[flag.Name] = value
	})

	// Order the fields in the form by the order they have in the struct
	var huhFields []huh.Field
	vtype := reflect.TypeOf(v).Elem()
	for i := 0; i < vtype.NumField(); i++ {
		// Get the field
		field := vtype.Field(i)

		// Get the cli tag
		cliTag := field.Tag.Get("cli")

		if huhField, ok := huhFieldMap[cliTag]; ok {
			huhFields = append(huhFields, huhField)
		}
	}

	return huh.NewForm(huh.NewGroup(huhFields...)), formValues

}
