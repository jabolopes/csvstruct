package csvstruct_test

import (
	"encoding/csv"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jabolopes/csvstruct"
)

const testData = `Character.Name,Character.Class,Attributes.HP,Attributes.Damage,Monster
Alex,Fighter,100,10,
Jayden,Wizard,90,20,
`

type Character struct {
	Name  string
	Class string
}

type Attributes struct {
	HP     int
	Damage int
}

type Monster struct{}

func ExampleReader() {
	reader := csvstruct.NewReader(csv.NewReader(strings.NewReader(testData)))
	reader.SetSchema([]interface{}{Character{}, Attributes{}, Monster{}})

	for {
		components, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		fmt.Printf("%#v\n", components[0].(Character))
		fmt.Printf("%#v\n", components[1].(Attributes))
		fmt.Printf("%#v\n", components[2].(Monster))
	}

	// Output: csvstruct_test.Character{Name:"Alex", Class:"Fighter"}
	// csvstruct_test.Attributes{HP:100, Damage:10}
	// csvstruct_test.Monster{}
	// csvstruct_test.Character{Name:"Jayden", Class:"Wizard"}
	// csvstruct_test.Attributes{HP:90, Damage:20}
	// csvstruct_test.Monster{}
}

func TestReader(t *testing.T) {
	want := [][]interface{}{
		[]interface{}{
			Character{"Alex", "Fighter"},
			Attributes{100, 10},
			Monster{},
		},
		[]interface{}{
			Character{"Jayden", "Wizard"},
			Attributes{90, 20},
			Monster{},
		},
	}

	reader := csvstruct.NewReader(csv.NewReader(strings.NewReader(testData)))
	reader.SetSchema([]interface{}{Character{}, Attributes{}, Monster{}})

	for {
		got, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read() err = %v; want %v", err, nil)
		}

		if diff := cmp.Diff(got, want[0]); diff != "" {
			t.Fatalf("Read() = %v; want %v\ndiff: %v", got, want[0], diff)
		}
		want = want[1:]
	}
}

type Prototype struct {
	Character  Character
	Attributes Attributes
	Monster    Monster
}

func TestReader_SetPrototypeSchema(t *testing.T) {
	want := [][]interface{}{
		[]interface{}{
			Character{"Alex", "Fighter"},
			Attributes{100, 10},
			Monster{},
		},
		[]interface{}{
			Character{"Jayden", "Wizard"},
			Attributes{90, 20},
			Monster{},
		},
	}

	reader := csvstruct.NewReader(csv.NewReader(strings.NewReader(testData)))
	reader.SetPrototypeSchema(reflect.TypeOf(Prototype{}))

	for {
		got, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read() err = %v; want %v", err, nil)
		}

		if diff := cmp.Diff(got, want[0]); diff != "" {
			t.Fatalf("Read() = %v; want %v\ndiff: %v", got, want[0], diff)
		}
		want = want[1:]
	}
}
