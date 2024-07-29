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
// 'MyComponent' and 'MyField'.
func parseHeaderColumnName(qualName string) (string, string, error) {
	splits := strings.SplitN(qualName, ".", 2)
	if len(splits) != 2 {
		return "", "", fmt.Errorf("expected qualified name, e.g. 'MyComponent.MyField'; got %v", qualName)
	}

	componentName := splits[0]
	fieldName := splits[1]
	return componentName, fieldName, nil
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

func (r *Reader) validateHeaderColumnName(componentName, fieldName string) (reflect.Type, error) {
	componentType, ok := r.schema[componentName]
	if !ok {
		return nil, fmt.Errorf("component type %q is not defined in the schema", componentName)
	}

	if _, ok := componentType.FieldByName(fieldName); !ok {
		return nil, fmt.Errorf("component type %q does not have field %q", componentName, fieldName)
	}

	return componentType, nil
}

func (r *Reader) createDescriptor(componentType reflect.Type, fieldName string) {
	// Create a resultDescriptor if needed.
	//
	// If a new component type is discovered, the resultDescriptor is extended
	// with information about that component.
	//
	// If the component type was already found in a previous column, then the
	// resultDescriptor is not updated.
	resultIndex := slices.Index(r.resultDescriptor.componentTypes, componentType)
	if resultIndex == -1 {
		resultIndex = len(r.resultDescriptor.componentTypes)
		r.resultDescriptor.componentTypes = append(r.resultDescriptor.componentTypes, componentType)
		r.resultDescriptor.componentValues = append(r.resultDescriptor.componentValues, reflect.New(componentType))
	}

	// Create a columnDescriptor.
	//
	// The column descriptor points back to the resultDescriptor via index.
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

		r.createDescriptor(componentType, fieldName)
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
		columnDescriptor := r.columnDescriptors[columnNum]

		value := r.resultDescriptor.componentValues[columnDescriptor.resultIndex]

		field := value.Elem().FieldByName(columnDescriptor.fieldName)
		if field.Kind() == reflect.Int {
			cellInt, _ := strconv.Atoi(cell)
			field.SetInt(int64(cellInt))
		} else if field.Kind() == reflect.String {
			field.SetString(cell)
		} else {
			return fmt.Errorf("unhandled kind %v", field.Kind())
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