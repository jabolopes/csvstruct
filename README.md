# csvstruct

[![PkgGoDev](https://pkg.go.dev/badge/github.com/jabolopes/go-ecs)](https://pkg.go.dev/github.com/jabolopes/csvstruct)

Import multiply-typed structured data from CSV to Go types.

Import spreadsheets:

![screenshot](https://github.com/user-attachments/assets/42f40d40-47ea-4d2e-89d3-34fabe38a528)

Into Go types:

```go
type Info struct { Name string }
type HP struct { HP int }
type Damage struct { Damage int }

type Prefab struct {
  Info *Info
  HP *HP
  Damage *Damage
}
```

Get typed data:

```go
Prefab{Info{"Death 1"}, HP{5}, Damage{100}}
Prefab{Info{"Death 2"}, HP{10}, Damage{200}}
Prefab{Info{"Death 3"}, HP{15}, Damage{300}}
```

When working with Google Sheets or Microsoft Excel, export data to CSV and
import it to your program using the csvstruct library.

## Example

Let's assume the following CSS file:

```css
Info.Name,Info.Class,Attributes.HP,Attributes.Damage
Alex,Fighter,100,10
Jayden,Wizard,90,20
Mary,Queen,,
...
```

The following program uses csvstruct library to import the data above:

```go
type Info struct {
    Name  string
    Class string
}

type Attributes struct {
    HP     int
    Damage int
}

type Prefab struct {
    Info       *Info
    Attributes *Attributes
}

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

    fmt.Printf("%v\n", prefab.Info)
    fmt.Printf("%v\n", prefab.Attributes)
}
```

## Format

The CSV data must have the following format:

### Header

The first row of the CSV data is the header and it must be present.

Each header column contains the name of a component followed by a period `.` and
a field name, e.g., `MyComponent.MyField`.

The `MyComponent.MyField` must be valid, i.e., `MyComponent` must be a valid
field name of the type `T` passed to `NewReader`, and `MyField` must be a valid
field of `MyComponent`.

If a cell is not given, then it's field is default initialized according to the
default initialization of Go. For example, pointers are default initialized to
`nil` and value types are default initialized to `0`, empty structs, empty
arrays, etc.

It's not required to put in the CSV header all the fields of
`MyComponent`. Rather, only the fields that should be imported by those CSV data
are present.

### Data rows

The rows that follow a CSV header are data rows.

The CSV data can contain 0 or more data rows.

The data rows must contain in each cell data that is compatible with
the type of the field specified in the CSV header.

For example, a field of type string can contain either an empty or
non-empty cell (without quotes) since that is compatible with the
string type.

A field of type `Int` can either an empty or non-empty cell containing
a numerical value.

Empty cells default initialize fields according to Go semantics.

### Multiple tables in the same CSV

It's possible to have multiple "tables" in the same CSV file. Tables are
separate by CSV header rows. The library caller must be able to determine when a
new CSV header is about to come up as the next row. In this case, the caller can
use `Reader.Clear` to start a new table of CSV data, followed by `Reader.Read`
to parse the new table.
