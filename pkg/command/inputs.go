package command

import (
	"fmt"
	"reflect"

	"github.com/spf13/cobra"
)

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

		elemField := elem.FieldByName(field.Name)

		switch field.Type.Kind() {
		case reflect.Ptr:
			switch field.Type.Elem().Kind() {
			case reflect.String:
				val, err := flags.GetString(cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(&val))
			case reflect.Int:
				val, err := flags.GetInt(cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(&val))
			case reflect.Float64:
				val, err := flags.GetFloat64(cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(&val))
			case reflect.Bool:
				val, err := flags.GetBool(cliTag)
				if err != nil {
					return err
				}
				elemField.Set(reflect.ValueOf(&val))
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
			val, err := flags.GetString(cliTag)
			if err != nil {
				return err
			}
			elemField.SetString(val)
		case reflect.Bool:
			val, err := flags.GetBool(cliTag)
			if err != nil {
				return err
			}
			elemField.SetBool(val)
		case reflect.Int:
			val, err := flags.GetInt(cliTag)
			if err != nil {
				return err
			}
			elemField.SetInt(int64(val))
		case reflect.Float64:
			val, err := flags.GetFloat64(cliTag)
			if err != nil {
				return err
			}
			elemField.SetFloat(val)
		default:
			return fmt.Errorf("unsupported type: %s", field.Type.Kind())
		}
	}

	return nil
}
