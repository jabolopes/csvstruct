package csvstruct

import (
	"encoding/csv"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
)

// Parses a qualified name, e.g., 'MyComponent.Myfield', into its parts, e.g.,
// 'MyComponent' and 'MyField'. It's also valid if the name only contains the
// component name without a field, e.g., 'MyComponent'.
func parseHeaderColumnName(qualName string) (string, string, error) {
	splits := strings.SplitN(qualName, ".", 2)
	if len(splits) == 1 {
		componentName := splits[0]
		fieldName := ""
		return componentName, fieldName, nil
	}

	if len(splits) != 2 {
		return "", "", fmt.Errorf("expected qualified name, e.g. 'MyComponent.MyField'; got %v", qualName)
	}

	componentName := splits[0]
	fieldName := splits[1]
	return componentName, fieldName, nil
}

func setFieldFromCell(field reflect.Value, cell string) error {
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(cell, 0, 64)
		if err != nil {
			return err
		}
		if field.OverflowInt(value) {
			return fmt.Errorf("cell %q contains a value that is too large for %s", cell, field.Kind())
		}
		field.SetInt(value)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(cell, 0, 64)
		if err != nil {
			return err
		}
		if field.OverflowUint(value) {
			return fmt.Errorf("cell %q contains a value that is too large for %s", cell, field.Kind())
		}
		field.SetUint(value)

	case reflect.Float32:
		value, err := strconv.ParseFloat(cell, 32)
		if err != nil {
			return err
		}
		field.SetFloat(value)

	case reflect.Float64:
		value, err := strconv.ParseFloat(cell, 64)
		if err != nil {
			return err
		}
		field.SetFloat(value)

	case reflect.String:
		field.SetString(cell)

	case reflect.Bool:
		value, err := strconv.ParseBool(cell)
		if err != nil {
			return err
		}
		field.SetBool(value)

	default:
		return fmt.Errorf("unhandled kind %s", field.Kind())
	}

	return nil
}

// resultDescriptor is the type information needed to parse row.
//
// Each slice is sorted in the order in which this will be returned to the
// caller.
//
// The order may coincidentally be sorted according to the CSV file if it
// happens that each column in the CSV defines a component type and a
// corresponding field once. In any case, don't rely on the order.
type resultDescriptor struct {
	// Types of each of the components that will n
	componentTypes  []reflect.Type
	componentValues []reflect.Value
	results         []interface{}
}

// columnDescriptor is the type information needed to associate a particular
// column with the data in the resultDescriptor.
type columnDescriptor struct {
	// Index into resultDescriptor.
	resultIndex int
	// Name of the field that a column stores.
	fieldName string
}

// Reader parses component data from CSV data.
//
// This is thread compatible, i.e., it's safe for non-concurrent use and it can
// be combined with external synchronization so it can be called concurrently.
type Reader struct {
	// Underlying CSV reader.
	reader *csv.Reader
	// Collection of types used to parse CSV data.
	schema map[string]reflect.Type
	// Permanent error. If there is one, it's returned on all Read calls.
	permanentErr error
	// Whether the descriptors have been computed.
	hasDescriptors bool
	// Descriptor for the results returned to the caller.
	resultDescriptor resultDescriptor
	// Descriptor for the columns, to parse from the CSV data.
	columnDescriptors []columnDescriptor
}

// Validates a pair of component and field names. The field name can be empty,
// in which case only the component name is validated.
func (r *Reader) validateHeaderColumnName(componentName, fieldName string) (reflect.Type, error) {
	componentType, ok := r.schema[componentName]
	if !ok {
		return nil, fmt.Errorf("component type %q is not defined in the schema", componentName)
	}

	if len(fieldName) > 0 {
		if _, ok := componentType.FieldByName(fieldName); !ok {
			return nil, fmt.Errorf("component type %q does not have field %q", componentName, fieldName)
		}
	}

	return componentType, nil
}

// Create a resultDescriptor if needed.
//
// If a new component type is discovered, the resultDescriptor is extended with
// information about that component.
//
// If the component type was already found in a previous column, then the
// resultDescriptor is not updated.
func (r *Reader) extendResultDescriptor(componentType reflect.Type) int {
	resultIndex := slices.Index(r.resultDescriptor.componentTypes, componentType)
	if resultIndex == -1 {
		resultIndex = len(r.resultDescriptor.componentTypes)
		r.resultDescriptor.componentTypes = append(r.resultDescriptor.componentTypes, componentType)
		r.resultDescriptor.componentValues = append(r.resultDescriptor.componentValues, reflect.New(componentType))
	}
	return resultIndex
}

// Create a columnDescriptor.
//
// The column descriptor points back to the resultDescriptor via index. The
// field name must not be empty.
func (r *Reader) createColumnDescriptor(resultIndex int, fieldName string) {
	r.columnDescriptors = append(r.columnDescriptors, columnDescriptor{resultIndex, fieldName})
}

func (r *Reader) createDescriptors(row []string) error {
	r.columnDescriptors = make([]columnDescriptor, 0, len(row))

	for _, qualName := range row {
		componentName, fieldName, err := parseHeaderColumnName(qualName)
		if err != nil {
			return err
		}

		componentType, err := r.validateHeaderColumnName(componentName, fieldName)
		if err != nil {
			return err
		}

		resultIndex := r.extendResultDescriptor(componentType)

		if len(fieldName) > 0 {
			r.createColumnDescriptor(resultIndex, fieldName)
		}
	}

	r.resultDescriptor.results = make([]interface{}, len(r.resultDescriptor.componentTypes))
	return nil
}

func (r *Reader) parseRow() error {
	row, err := r.reader.Read()
	if err != nil {
		return err
	}

	// Zero all values because they are reused across reads.
	for i := range r.resultDescriptor.componentValues {
		r.resultDescriptor.componentValues[i].Elem().SetZero()
	}

	for columnNum, cell := range row {
		if len(cell) == 0 {
			continue
		}

		columnDescriptor := r.columnDescriptors[columnNum]

		value := r.resultDescriptor.componentValues[columnDescriptor.resultIndex]

		field := value.Elem().FieldByName(columnDescriptor.fieldName)
		if err := setFieldFromCell(field, cell); err != nil {
			return err
		}
	}

	return nil
}

// SetSchema tells the Reader which component types to use when parsing CSV
// data.
//
// It's allowed to give more types than those that will actually be used for
// parsing.
//
// It's allowed to call this function multiple times, even after parsing has
// started.
func (r *Reader) SetSchema(components []interface{}) {
	schema := map[string]reflect.Type{}
	for _, component := range components {
		// TODO: Check type is struct.
		// TODO: Check duplicates.
		typ := reflect.TypeOf(component)
		schema[typ.Name()] = typ
	}
	r.schema = schema
}

// SetPrototypeSchema is an alternative approach to defining the schema. The
// prototype is a struct type that contains fields, where the field name should
// correspond to the component name that appears in the CSV header, and the
// field type corresponds to the Go type that will be parsed and returned to the
// caller.
//
// Example:
//
//   data.csv:
//     CharacterInfo.Name,CharacterHP.HP
//     Alex,100
//     Jayden,120
//
//   program.go:
//     type CharacterInfo struct { Name string }
//     type CharacterHP struct { HP int }
//
//     type Prototype struct {
//       CharacterInfo CharacterInfo
//       CharacterHP CharacterHP
//     }
//
//     reader.SetPrototypeSchema(reflect.TypeOf(Prototype{}))
func (r *Reader) SetPrototypeSchema(typ reflect.Type) {
	schema := map[string]reflect.Type{}
	fields := reflect.VisibleFields(typ)
	for _, field := range fields {
		schema[field.Name] = field.Type
	}
	r.schema = schema
}

// Clears part of the internal state so that this is ready to continue parsing,
// namely, it clears the permanent error and all the internal descriptors. After
// Clear() is called, Read() will expect the next row to be a CSV header. This
// is useful if the same CSV file contains multiple tables of data.
func (r *Reader) Clear() {
	r.permanentErr = nil
	r.hasDescriptors = false
	r.resultDescriptor = resultDescriptor{}
	r.columnDescriptors = nil
}

// Reads the next CSV row and returns typed data.
//
// It's expected that the first row is the CSV header. This header is used to
// construct the internal type descriptors that will be used to parse CSV data
// into Go types.
//
// After Clear() has been called, reading can resume but it's again expected
// that the next row is a header row.
//
// Returns io.EOF when the end of file is reached. When an error is returned,
// the first return value is always nil. In other words, this either returns
// valid data or it returns an error, but never both simultaneously.
//
// IMPORTANT: Retains ownership of the returned `[]interface{}` value and
// subsequent calls to `Read` may overwrite these values. As such, if the caller
// intends to retain these values, the caller should copy them.
func (r *Reader) Read() ([]interface{}, error) {
	if r.permanentErr != nil {
		return nil, r.permanentErr
	}

	if !r.hasDescriptors {
		row, err := r.reader.Read()
		if err == io.EOF {
			return nil, fmt.Errorf("failed to read CSV header: %v", err)
		}
		if err != nil {
			return nil, err
		}

		if err := r.createDescriptors(row); err != nil {
			r.Clear()
			r.permanentErr = err
			return nil, err
		}

		r.hasDescriptors = true
	}

	// Read a CSV row and parse it based on the descriptors.
	if err := r.parseRow(); err == io.EOF {
		r.Clear()
		r.permanentErr = err
		return nil, err
	} else if err != nil {
		r.Clear()
		r.permanentErr = err
		return nil, err
	}

	// Return results from the last read.
	for i, value := range r.resultDescriptor.componentValues {
		r.resultDescriptor.results[i] = value.Elem().Interface()
	}

	return r.resultDescriptor.results, nil
}

// NewReader returns a new reader using the given `reader` as the underlying CSV
// reader. After this is called, SetSchema() should be called, and only
// afterwards Read().
func NewReader(reader *csv.Reader) *Reader {
	reader.ReuseRecord = true
	return &Reader{reader: reader}
}
