package csvstruct

import (
	"encoding/csv"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
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

type colDescriptor struct {
	kind          reflect.Kind
	componentName string
	fieldName     string
}

// Reader parses component data from CSV data.
//
// This is thread compatible, i.e., it's safe for non-concurrent use and it can
// be combined with external synchronization so it can be called concurrently.
type Reader[T any] struct {
	// Underlying CSV reader.
	reader *csv.Reader
	// Permanent error. If there is one, it's returned on all Read calls.
	permanentErr error
	// Whether the descriptors have been computed.
	hasDescriptors bool
	// Column descriptor.
	colDescriptors []colDescriptor
}

// createDescriptors creates the column descriptors from the CSV header.
func (r *Reader[T]) createDescriptors(row []string) error {
	r.colDescriptors = make([]colDescriptor, 0, len(row))

	for _, qualName := range row {
		componentName, fieldName, err := parseHeaderColumnName(qualName)
		if err != nil {
			return err
		}

		field, ok := reflect.TypeFor[T]().FieldByName(componentName)
		if !ok {
			return fmt.Errorf("type %s does not have a field %q", reflect.TypeFor[T]().String(), componentName)
		}

		var kind reflect.Kind
		if len(fieldName) > 0 {
			subfield, ok := field.Type.Elem().FieldByName(fieldName)
			if !ok {
				return fmt.Errorf("type %s does not have a field %q", field.Type.String(), fieldName)
			}
			kind = subfield.Type.Kind()
		}

		r.colDescriptors = append(r.colDescriptors, colDescriptor{kind, componentName, fieldName})
	}

	return nil
}

// parseRow parses a data row into `t`.
func (r *Reader[T]) parseRow(t *T) error {
	row, err := r.reader.Read()
	if err != nil {
		return err
	}

	var def T
	*t = def

	data := map[string]interface{}{}
	for columnNum, cell := range row {
		if len(cell) == 0 {
			continue
		}

		descriptor := r.colDescriptors[columnNum]

		var value interface{}
		switch descriptor.kind {
		case reflect.Int, reflect.Int32, reflect.Int64:
			number, err := strconv.Atoi(cell)
			if err != nil {
				return err
			}
			value = number
		case reflect.String:
			value = cell
		}

		if obj, ok := data[descriptor.componentName]; ok {
			obj.(map[string]interface{})[descriptor.fieldName] = value
		} else {
			data[descriptor.componentName] = map[string]interface{}{descriptor.fieldName: value}
		}
	}

	return mapstructure.Decode(data, t)
}

// Clears part of the internal state so that this is ready to continue parsing,
// namely, it clears the permanent error and all the internal descriptors. After
// Clear() is called, Read() will expect the next row to be a CSV header. This
// is useful if the same CSV file contains multiple tables of data.
func (r *Reader[T]) Clear() {
	r.permanentErr = nil
	r.hasDescriptors = false
	r.colDescriptors = nil
}

// Reads the next CSV row and returns typed data.
//
// It's expected that the first row is the CSV header. This header is used to
// construct the column descriptors that will be used to direct column parsing.
//
// If Clear() has been called, reading can resume and it's once again expected
// that the next row is a CSV header row.
//
// Returns io.EOF when the end of file is reached. When an error is returned,
// the first return value is always nil. In other words, this either returns
// valid data or it returns an error, but never both simultaneously.
func (r *Reader[T]) Read(t *T) error {
	if r.permanentErr != nil {
		return r.permanentErr
	}

	if !r.hasDescriptors {
		row, err := r.reader.Read()
		if err == io.EOF {
			return fmt.Errorf("failed to read CSV header: %v", err)
		}
		if err != nil {
			return err
		}

		if err := r.createDescriptors(row); err != nil {
			r.Clear()
			r.permanentErr = err
			return err
		}

		r.hasDescriptors = true
	}

	// Read a CSV row and parse it based on the descriptors.
	if err := r.parseRow(t); err == io.EOF {
		r.Clear()
		r.permanentErr = err
		return err
	} else if err != nil {
		r.Clear()
		r.permanentErr = err
		return err
	}

	return nil
}

// NewReader returns a new reader using the given `reader` as the underlying CSV
// reader. The type `T` is the schema that is used to parse the data.
func NewReader[T any](reader *csv.Reader) *Reader[T] {
	reader.ReuseRecord = true
	csvreader := &Reader[T]{reader: reader}
	return csvreader
}
