package csvstruct_test

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jabolopes/csvstruct"
)

const testData = `Character.Name,Character.Class,Attributes.HP,Attributes.Damage
Alex,Fighter,100,10
Jayden,Wizard,90,20
`

type Character struct {
	Name  string
	Class string
}

type Attributes struct {
	HP     int
	Damage int
}

func ExampleReader() {
	reader := csvstruct.NewReader(csv.NewReader(strings.NewReader(testData)))
	reader.SetSchema([]interface{}{Character{}, Attributes{}})

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
	}

	// Output: csvstruct_test.Character{Name:"Alex", Class:"Fighter"}
	// csvstruct_test.Attributes{HP:100, Damage:10}
	// csvstruct_test.Character{Name:"Jayden", Class:"Wizard"}
	// csvstruct_test.Attributes{HP:90, Damage:20}
}

func TestReader(t *testing.T) {
	want := [][]interface{}{
		[]interface{}{
			Character{"Alex", "Fighter"},
			Attributes{100, 10},
		},
		[]interface{}{
			Character{"Jayden", "Wizard"},
			Attributes{90, 20},
		},
	}

	reader := csvstruct.NewReader(csv.NewReader(strings.NewReader(testData)))
	reader.SetSchema([]interface{}{Character{}, Attributes{}})

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
