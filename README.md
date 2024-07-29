# csvstruct

[![PkgGoDev](https://pkg.go.dev/badge/github.com/jabolopes/go-ecs)](https://pkg.go.dev/github.com/jabolopes/csvstruct)

Import multiply-typed structured data from CSV to Go types.

Import spreadsheets:

![screenshot](https://github.com/user-attachments/assets/42f40d40-47ea-4d2e-89d3-34fabe38a528)

Into Go types:

```go
type CharacterInfo struct { Name string }
type CharacterHP struct { HP int }
type CharacterDamage struct { Damage int }
```

Get typed data:

```go
CharacterInfo{"Death 1"}, CharacterInfo{"Death 2"}, CharacterInfo{"Death 3"}, ...
CharacterHP{5}, CharacterHP{10}, CharacterHP{15}, ...
CharacterDamage{100}, CharacterDamage{200}, CharacterDamage{300}, ...
```

When working with Google Sheets or Microsoft Excel, export data to CSV and import it to your program using the csvstruct library.

## Example

Let's assume the following CSS file:

```css
Character.Name,Character.Class,Attributes.HP,Attributes.Damage
Alex,Fighter,100,10
Jayden,Wizard,90,20
...
```

The following program uses csvstruct library to import the data above:

```go
type Character struct {
    Name  string
    Class string
}

type Attributes struct {
    HP     int
    Damage int
}

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

    // components[0] contains Character, e.g., {"Alex", "Fighter"}, {"Jayden", "Wizard"}, etc.
    // components[1] contains Attributes, e.g., {100, 10}, {90, 20}, etc.
    
    // The components have the correct types, e.g., the following assertions are true:
    components[0].(Character)  // True.
    components[1].(Attributes)  // True.
}
```

## Format

The CSV data must have the following format:

### Header

The first row of the CSV data is the header and it must be present.

Each header column contains the name of a component followed by a
period `.`, followed by a field name, e.g., `MyComponent.MyField`.

The `MyComponent` must correspond to a Go type with the same
name. Package names associated with the type are not used. For
example, if the full Go type name is `mypackage.MyComponent`, only
`MyComponent` is used.

The `MyField` must be a valid field of `MyComponent` and it must be
capitalized (i.e., exported).

It's not required to put in the CSV header all the fields of
`MyComponent`. Rather, only the fields that should be imported by
those CSV data are present.

It's possible to have multiple CSV headers. The library caller must be
able to determine when a new CSV header is about to come up as the
next row. In this case, the caller can use `Reader.Clear`, optionally
followed by `Reader.SetSchema`, to start a new table of CSV data.

### Data rows

The rows that follow a CSV header are data rows.

The CSV data can contain 0 or more data rows.

The data rows must contain in each cell data that is compatible with
the type of the field specified in the CSV header.

For example, a field of type string can contain either an empty or
non-empty cell (without quotes) since that is compatible with the
string type.

A field of type `Int` must contain a non-empty numerical cell.
