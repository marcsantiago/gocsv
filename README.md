# GoCSV

[![Go Reference](https://pkg.go.dev/badge/github.com/marcsantiago/gocsv.svg)](https://pkg.go.dev/github.com/marcsantiago/gocsv)

GoCSV maps CSV records to Go structs and serializes Go structs back to CSV. It supports struct tags, nested structs, custom value conversion, custom CSV readers and writers, and row-by-row processing.

## Requirements

Go 1.27 or newer.

## Installation

Add GoCSV to your module:

```sh
go get github.com/marcsantiago/gocsv
```

## Quick start

Given this CSV data:

```csv
client_id,client_name,client_age
1,Jose,42
2,Daniel,26
3,Vincent,32
```

Map it to a struct using `csv` tags, then serialize the records back to CSV:

```go
package main

import (
	"fmt"

	"github.com/marcsantiago/gocsv"
)

type Client struct {
	ID   int    `csv:"client_id"`
	Name string `csv:"client_name"`
	Age  int    `csv:"client_age"`
}

func main() {
	input := "client_id,client_name,client_age\n" +
		"1,Jose,42\n" +
		"2,Daniel,26\n" +
		"3,Vincent,32\n"

	var clients []Client
	if err := gocsv.UnmarshalString(input, &clients); err != nil {
		panic(err)
	}

	for _, client := range clients {
		fmt.Printf("Hello %s\n", client.Name)
	}

	clients = append(clients, Client{ID: 4, Name: "John", Age: 21})

	output, err := gocsv.MarshalString(&clients)
	if err != nil {
		panic(err)
	}
	fmt.Print(output)
}
```

Use `Unmarshal`, `UnmarshalBytes`, or `UnmarshalFile` when the input comes from an `io.Reader`, a byte slice, or an `*os.File`. Use `Marshal`, `MarshalBytes`, or `MarshalFile` to write CSV to an `io.Writer`, a byte slice, or a file.

## Struct tags

GoCSV uses the `csv` struct tag by default. An exported field without a tag uses its Go field name as its column name.

```go
type Client struct {
	ID            int    `csv:"client_id"`
	Name          string `csv:"client_name"`
	Nickname      string `csv:"nickname,omitempty"`
	Country       string `csv:"country,partial"`
	Status        string `csv:"status,default=active"`
	InternalValue string `csv:"-"`
}
```

The tags support these options:

| Tag                           | Behavior                                                                                                                              |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `csv:"client_id"`             | Use `client_id` as the column name.                                                                                                   |
| `csv:"-"`                     | Ignore the field.                                                                                                                     |
| `csv:"nickname,omitempty"`    | When decoding an empty value into a pointer field, leave the leaf pointer unset. This does not remove the column from the CSV output. |
| `csv:"status,default=active"` | When decoding an empty cell, use `active` as the value.                                                                               |
| `csv:"country,partial"`       | Match a header that contains `country`, such as `publisher_country`.                                                                  |

Tag options are separated by commas by default. Matching is exact unless the `partial` option or a custom header normalizer is used.

### Nested structs

Nested structs are expanded into columns. By default, the parent field name is prefixed to each child field name:

```go
type Foo struct {
	B Bar
	X int `csv:"x"`
}

type Bar struct {
	A int `csv:"a"`
	B int `csv:"b"`
}
```

The columns are `B.a`, `B.b`, and `x`.

Use `csv:"."` to inline a nested struct without the parent prefix:

```go
type Foo struct {
	B Bar `csv:"."`
	X int `csv:"x"`
}
```

The columns are `a`, `b`, and `x`.

The default nested-field separator is `.`. The package-level `gocsv.FieldsCombiner` setting changes this separator. Set it before GoCSV first processes the relevant struct types: field metadata is cached. When decoding nested pointer structs, GoCSV allocates the pointers needed to reach a populated field. An `omitempty` option can leave an empty leaf pointer unset, but it does not prevent allocation of enclosing structs.

## Custom value conversion

Implement `MarshalCSV` and `UnmarshalCSV` when a type needs a representation different from its default Go value. GoCSV also supports `encoding.TextMarshaler`, `encoding.TextUnmarshaler`, and `fmt.Stringer` for marshaling.

```go
package main

import "time"

type DateTime struct {
	time.Time
}

func (d DateTime) MarshalCSV() (string, error) {
	return d.Time.Format("20060102"), nil
}

func (d *DateTime) UnmarshalCSV(value string) error {
	parsed, err := time.Parse("20060102", value)
	if err != nil {
		return err
	}
	d.Time = parsed
	return nil
}

type Client struct {
	ID       int      `csv:"id"`
	Name     string   `csv:"name"`
	Employed DateTime `csv:"employed"`
}
```

## Customizing complete records

For record types that need to interpret the header and each cell themselves, implement `UnmarshalCSVWithFields`. GoCSV calls the method for each header/value pair and uses it instead of its normal tag-to-field assignment for that record type.

```go
type Record struct {
	Values map[string]string
}

func (r *Record) UnmarshalCSVWithFields(key, value string) error {
	if r.Values == nil {
		r.Values = make(map[string]string)
	}
	r.Values[key] = value
	return nil
}
```

The method has this signature:

```go
UnmarshalCSVWithFields(key, value string) error
```

Return an error when a value cannot be handled. GoCSV wraps conversion errors with CSV row and column information.

## Transforming values before decoding

Use a custom `Decoder` with `UnmarshalDecoder` to normalize data after CSV parsing but before values are assigned to struct fields. This is useful when a source uses a sentinel value that Go cannot convert directly—for example, mapping `N/A` to `NaN` for a floating-point `Height` column without changing the same text in other columns.

See the [runnable sanitizing example](sanitize_example_test.go). It preserves the header row and changes only data cells in the configured column. Quoted fields, Unicode, and ordinary conversion errors continue to be handled by the CSV reader and GoCSV as usual.

A decoder based on `csv.Reader.ReadAll` holds all records in memory. For row-by-row processing, implement a `SimpleDecoder` with `GetCSVRow` and `GetCSVRows`, then use `UnmarshalDecoderToChan` or `UnmarshalDecoderToCallback`.

## Streaming large CSV files

If you do not need all records in memory at once, use the channel or callback APIs. For example:

```go
err := gocsv.UnmarshalToCallback(inputReader, func(client Client) {
	fmt.Println(client.Name)
})
if err != nil {
	panic(err)
}
```

The callback must accept one record value. It may also return an `error`; a returned error is propagated after the parser has been drained. For channel-based processing, use `UnmarshalToChan` or `UnmarshalDecoderToChan` with a channel whose element type is your record type.

## CSV without a header row

Use the `WithoutHeaders` variants when the input or output has no header row:

```go
if err := gocsv.UnmarshalWithoutHeaders(inputReader, &clients); err != nil {
	panic(err)
}

output, err := gocsv.MarshalStringWithoutHeaders(&clients)
if err != nil {
	panic(err)
}
```

## Custom CSV reader and writer

GoCSV uses the standard library's `encoding/csv` reader and writer by default. Set a custom reader or writer when the file uses another delimiter or requires different parsing behavior.

```go
func configurePipeDelimitedCSV() {
	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		reader := csv.NewReader(in)
		reader.Comma = '|'
		return reader
	})

	gocsv.SetCSVWriter(func(out io.Writer) *gocsv.SafeCSVWriter {
		writer := csv.NewWriter(out)
		writer.Comma = '|'
		return gocsv.NewSafeCSVWriter(writer)
	})
}
```

Call `configurePipeDelimitedCSV` during application startup, before reading or writing CSV. `SetCSVReader` and `SetCSVWriter` change package-level configuration; they are not per-call settings. `gocsv.LazyCSVReader` is also available when you want `LazyQuotes` and `TrimLeadingSpace` enabled.

## Header matching and validation

The package offers global settings for applications that need stricter validation or different header matching:

- `gocsv.SetHeaderNormalizer(fn)` normalizes both CSV headers and struct field names before matching. For example, `gocsv.SetHeaderNormalizer(strings.ToLower)` enables case-insensitive matching. This setting can be changed safely while other goroutines are reading or writing CSV.
- `gocsv.FailIfUnmatchedStructTags = true` returns an error when expected struct fields do not match CSV headers.
- `gocsv.FailIfDoubleHeaderNames = true` returns an error when the CSV header contains duplicate names.
- `gocsv.ShouldAlignDuplicateHeadersWithStructFieldOrder = true` aligns repeated header names with matching struct fields in declaration order.

These are package-level settings. Configure them deliberately when initializing the application.

## API reference

See the [Go reference documentation](https://pkg.go.dev/github.com/marcsantiago/gocsv) for the complete API.
