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

const testData = `Info.Name,Info.Class,Attributes.HP,Attributes.Damage,Player
Alex,Fighter,100,10,
Jayden,Wizard,90,20,
Mary,Queen,,,
Player,,,,0
`

type Info struct {
	Name  string
	Class string
}

type Attributes struct {
	HP     int
	Damage int
}

type Player struct{}

type Prefab struct {
	Info       *Info
	Attributes *Attributes
	Player     *Player
}

func ExampleReader() {
	reader := csvstruct.NewReader[Prefab](csv.NewReader(strings.NewReader(testData)))

	var prefab Prefab
	for {
		err := reader.Read(&prefab)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		fmt.Printf("%#v\n", prefab.Info)
		fmt.Printf("%#v\n", prefab.Attributes)
		fmt.Printf("%#v\n", prefab.Player)
	}

	// Output: &csvstruct_test.Info{Name:"Alex", Class:"Fighter"}
	// &csvstruct_test.Attributes{HP:100, Damage:10}
	// (*csvstruct_test.Player)(nil)
	// &csvstruct_test.Info{Name:"Jayden", Class:"Wizard"}
	// &csvstruct_test.Attributes{HP:90, Damage:20}
	// (*csvstruct_test.Player)(nil)
	// &csvstruct_test.Info{Name:"Mary", Class:"Queen"}
	// (*csvstruct_test.Attributes)(nil)
	// (*csvstruct_test.Player)(nil)
	// &csvstruct_test.Info{Name:"Player", Class:""}
	// (*csvstruct_test.Attributes)(nil)
	// &csvstruct_test.Player{}
}

func TestReader(t *testing.T) {
	want := []Prefab{
		Prefab{
			&Info{"Alex", "Fighter"},
			&Attributes{100, 10},
			nil,
		},
		Prefab{
			&Info{"Jayden", "Wizard"},
			&Attributes{90, 20},
			nil,
		},
		Prefab{
			&Info{"Mary", "Queen"},
			nil,
			nil,
		},
		Prefab{
			&Info{"Player", ""},
			nil,
			&Player{},
		},
	}

	reader := csvstruct.NewReader[Prefab](csv.NewReader(strings.NewReader(testData)))

	var got Prefab
	for {
		err := reader.Read(&got)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read() err = %v; want %v", err, nil)
		}

		if diff := cmp.Diff(want[0], got); diff != "" {
			t.Fatalf("Read() diff = %v", diff)
		}
		want = want[1:]
	}
}
