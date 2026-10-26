# GoCSV

[![Go Reference](https://pkg.go.dev/badge/github.com/marcsantiago/gocsv.svg)](https://pkg.go.dev/github.com/marcsantiago/gocsv)

GoCSV maps CSV records to Go structs and writes Go structs back to CSV. It supports struct tags, nested structs, custom value conversion, configurable CSV readers and writers, and streaming records without loading an entire file into memory.

## Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Struct tags](#struct-tags)
- [Nested structs](#nested-structs)
- [Per-call options](#per-call-options)
- [Custom value conversion](#custom-value-conversion)
- [Custom record decoding](#custom-record-decoding)
- [Transforming values before decoding](#transforming-values-before-decoding)
- [Streaming large CSV files](#streaming-large-csv-files)
- [CSV without headers](#csv-without-headers)
- [Custom CSV readers and writers](#custom-csv-readers-and-writers)
- [Header matching](#header-matching)
- [API reference](#api-reference)

## Requirements

Go 1.27 or newer.

## Installation

```sh
go get github.com/marcsantiago/gocsv
```

## Quick start

Given a CSV file with a header row:

```csv
client_id,client_name,client_age
1,Jose,42
2,Daniel,26
3,Vincent,32
```

Define a struct with `csv` tags, then unmarshal and marshal the records:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/marcsantiago/gocsv"
)

type Client struct {
	ID   int    `csv:"client_id"`
	Name string `csv:"client_name"`
	Age  int    `csv:"client_age"`
}

func main() {
	input := strings.NewReader("client_id,client_name,client_age\n" +
		"1,Jose,42\n" +
		"2,Daniel,26\n" +
		"3,Vincent,32\n")

	var clients []Client
	if err := gocsv.Unmarshal(input, &clients); err != nil {
		panic(err)
	}

	for _, client := range clients {
		fmt.Printf("Hello %s\n", client.Name)
	}

	clients = append(clients, Client{ID: 4, Name: "John", Age: 21})

	var output strings.Builder
	if err := gocsv.Marshal(&clients, &output); err != nil {
		panic(err)
	}
	fmt.Print(output.String())
}
```

Use `Unmarshal` for an `io.Reader`, `UnmarshalString` for a string, `UnmarshalBytes` for a byte slice, or `UnmarshalFile` for a file. The corresponding `Marshal` functions write to an `io.Writer`, return a string or byte slice, or write to a file.

## Struct tags

GoCSV uses the `csv` struct tag to name columns. If a field has no `csv` tag, its Go field name is used. Tag options are separated by commas.

```go
type Client struct {
	ID            int    `csv:"client_id"`
	Nickname      *string `csv:"nickname,omitempty"`
	Country       string `csv:"country,partial"`
	Status        string `csv:"status,default=active"`
	InternalValue string `csv:"-"`
}
```

| Tag                           | Behavior                                                                                                                   |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `csv:"client_id"`             | Use `client_id` as the column name.                                                                                        |
| `csv:"-"`                     | Ignore the field.                                                                                                          |
| `csv:"nickname,omitempty"`    | When decoding an empty cell into a pointer field, leave the leaf pointer unset. The column is not removed from CSV output. |
| `csv:"status,default=active"` | Use `active` when decoding an empty cell.                                                                                  |
| `csv:"country,partial"`       | Match a header containing `country`, such as `publisher_country`.                                                          |

Header matching is exact by default, after any configured header normalization. The `partial` option allows substring matching for that field.

## Nested structs

GoCSV expands nested structs into columns. By default, the parent field name is prefixed to each child field name, using `.` as the separator.

```go
type Address struct {
	City    string `csv:"city"`
	Country string `csv:"country"`
}

type Client struct {
	Name    string  `csv:"name"`
	Address Address
}
```

The nested columns are `Address.city` and `Address.country`, alongside `name`.

Use `csv:"."` to inline a nested struct and omit the parent prefix:

```go
type Client struct {
	Name    string  `csv:"name"`
	Address Address `csv:"."`
}
```

The resulting columns are `name`, `city`, and `country`.

When decoding nested pointer structs, GoCSV allocates the enclosing pointers needed to reach a populated field. `omitempty` can leave an empty leaf pointer unset, but does not prevent allocation of enclosing structs.

## Per-call options

Functions that accept `...OptionModifier` can be configured per call, instead of relying on package-level flags for these behaviors.

```go
err := gocsv.Unmarshal(
	inputReader,
	&clients,
	gocsv.WithUnmatchTagFailure(true),
	gocsv.WithDuplicateHeaderFailure(true),
)
if err != nil {
	panic(err)
}
```

Available options:

| Option                             | Default | Behavior                                                                       |
| ---------------------------------- | ------- | ------------------------------------------------------------------------------ |
| `WithFieldsCombiner(separator)`    | `"."`   | Sets the separator used between parent and nested field names.                 |
| `WithUnmatchTagFailure(true)`      | `false` | Returns an error when a tagged struct field does not match a CSV header.       |
| `WithDuplicateHeaderFailure(true)` | `false` | Returns an error when the CSV header contains duplicate names.                 |
| `WithAlignDuplicateHeader(true)`   | `false` | Aligns repeated header names with matching struct fields in declaration order. |

For example, `gocsv.Unmarshal(inputReader, &clients, gocsv.WithFieldsCombiner("_"))` requests nested columns such as `Address_city` instead of `Address.city`. These options are supported by the core marshal/unmarshal functions and the streaming channel APIs that accept `...OptionModifier`; check the function signature when using a convenience wrapper.

## Custom value conversion

Implement `MarshalCSV` and `UnmarshalCSV` when a type needs a representation different from its default Go value. GoCSV also honors `encoding.TextMarshaler` and `fmt.Stringer` when marshaling, and `encoding.TextUnmarshaler` when unmarshaling.

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

## Custom record decoding

If a record needs to interpret column names and values itself, implement `UnmarshalCSVWithFields`. GoCSV calls the method for each header/value pair instead of assigning those values through the record's struct tags.

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

The method signature is:

```go
UnmarshalCSVWithFields(key, value string) error
```

Return an error when a value cannot be handled. GoCSV wraps conversion errors with CSV row and column information.

## Transforming values before decoding

Use a custom `Decoder` with `UnmarshalDecoder` to normalize values after CSV parsing but before they are assigned to struct fields. This is useful when a source uses a sentinel value that Go cannot convert directly—for example, mapping `N/A` to `NaN` for a floating-point column without changing the same text in other columns.

See the [runnable sanitizing example](sanitize_example_test.go). It preserves the header row and changes only data cells in the configured column. CSV quoting, Unicode, and ordinary conversion errors continue to be handled by the CSV reader and GoCSV.

A decoder based on `csv.Reader.ReadAll` holds every record in memory. For row-by-row processing, implement a `SimpleDecoder` with `GetCSVRow` and `GetCSVRows`, then use `UnmarshalDecoderToChan` or `UnmarshalDecoderToCallback`.

## Streaming large CSV files

To process records without collecting them all in a slice, use the channel or callback APIs.

```go
err := gocsv.UnmarshalToCallback(inputReader, func(client Client) error {
	fmt.Println(client.Name)
	return nil
})
if err != nil {
	panic(err)
}
```

The callback receives one record at a time. A callback may return an error; GoCSV stops invoking it after the first callback error, drains the parser, and returns that error. For channel-based processing, use `UnmarshalToChan` or `UnmarshalDecoderToChan` with a channel whose element type matches the record type.

## CSV without headers

Use the `WithoutHeaders` variants when the input or output has no header row:

```go
if err := gocsv.UnmarshalWithoutHeaders(inputReader, &clients); err != nil {
	panic(err)
}

if err := gocsv.MarshalWithoutHeaders(&clients, outputWriter); err != nil {
	panic(err)
}
```

## Custom CSV readers and writers

GoCSV uses the standard library's `encoding/csv` reader and writer by default. Configure a custom reader or writer when a file uses another delimiter or needs different parsing behavior.

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

Call this during application startup, before reading or writing CSV. `SetCSVReader` and `SetCSVWriter` change package-level configuration, not per-call settings. `gocsv.LazyCSVReader` is available when `LazyQuotes` and `TrimLeadingSpace` should be enabled.

## Header matching

Use `SetHeaderNormalizer` to normalize CSV headers and struct field names before matching. For example, `gocsv.SetHeaderNormalizer(strings.ToLower)` enables case-insensitive matching.

```go
gocsv.SetHeaderNormalizer(strings.ToLower)
```

The normalizer is package-level configuration and may be changed safely while other goroutines are reading or writing CSV. Set it deliberately during application initialization if every call should share the same matching behavior. Use the per-call validation options described above when stricter matching should apply to selected operations.

## API reference

See the [Go reference documentation](https://pkg.go.dev/github.com/marcsantiago/gocsv) for the complete API.
