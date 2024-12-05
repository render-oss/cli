package command

import (
	"fmt"
	"strings"
)

const (
	EnumType = "enum"
)

type CobraEnum struct {
	values          []string
	selectedIndexes []int
	isMultiSelect   bool
}

func NewEnumInput(values []string, isMultiSelect bool) *CobraEnum {
	return &CobraEnum{
		values:        values,
		isMultiSelect: isMultiSelect,
	}
}

func (e *CobraEnum) String() string {
	if len(e.values) == 0 && !e.isMultiSelect {
		return "Invalid enum value"
	}

	values := make([]string, len(e.selectedIndexes))
	for i, index := range e.selectedIndexes {
		values[i] = e.values[index]
	}

	return strings.Join(values, ", ")
}

func (e *CobraEnum) Set(v string) error {
	values := strings.Split(v, ",")

	for _, splitValue := range values {
		for i, value := range e.values {
			if strings.EqualFold(splitValue, value) {
				e.selectedIndexes = append(e.selectedIndexes, i)
			}
		}
	}

	if len(e.selectedIndexes) != 0 {
		return nil
	}

	var stringValues []string

	for _, value := range e.values {
		stringValues = append(stringValues, fmt.Sprintf("%q", value))
	}

	return fmt.Errorf("must be one of %s", strings.Join(stringValues, ", "))
}

func (e *CobraEnum) Type() string {
	return EnumType
}

func (e *CobraEnum) Values() []string {
	return e.values
}

func (e *CobraEnum) SelectedValues() []string {
	var selectedValues []string
	for _, index := range e.selectedIndexes {
		selectedValues = append(selectedValues, e.values[index])
	}
	return selectedValues
}

func (e *CobraEnum) IsMultiSelect() bool {
	return e.isMultiSelect
}
