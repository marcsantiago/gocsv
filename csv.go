package gocsv

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
)

const (
	// TagName defines key in the struct field's tag to scan
	TagName = "csv"
	// TagSeparator defines separator string for multiple csv tags in struct fields
	TagSeparator = ","
)

// Normalizer is a function that takes and returns a string. It is applied to
// struct and header field values before they are compared. It can be used to alter
// names for comparison. For instance, you could allow case insensitive matching
// or convert '-' to '_'.
type Normalizer func(string) string

type ErrorHandler func(*csv.ParseError) bool

// headerNormalization is the Normalizer in use together with the struct infos
// built with it. SetHeaderNormalizer replaces both at once, so it is safe to
// call while other goroutines are reading or writing CSV.
type headerNormalization struct {
	normalize Normalizer
}

// currentHeaderNormalization holds a *headerNormalization, initially with a nop Normalizer.
var currentHeaderNormalization atomic.Value

func init() {
	currentHeaderNormalization.Store(&headerNormalization{normalize: DefaultNameNormalizer()})
}

func loadHeaderNormalization() *headerNormalization {
	return currentHeaderNormalization.Load().(*headerNormalization)
}

func normalizeName(name string) string {
	return loadHeaderNormalization().normalize(name)
}

// DefaultNameNormalizer is a nop Normalizer.
func DefaultNameNormalizer() Normalizer { return func(s string) string { return s } }

// SetHeaderNormalizer sets the normalizer used to normalize struct and header field names.
func SetHeaderNormalizer(f Normalizer) {
	// The struct infos are rebuilt from scratch, as they hold normalized names.
	currentHeaderNormalization.Store(&headerNormalization{normalize: f})
}

// --------------------------------------------------------------------------
// CSVWriter used to format CSV

var selfCSVWriter = DefaultCSVWriter

// DefaultCSVWriter is the default SafeCSVWriter used to format CSV (cf. csv.NewWriter)
func DefaultCSVWriter(out io.Writer) *SafeCSVWriter {
	writer := NewSafeCSVWriter(csv.NewWriter(out))

	// As only one rune can be defined as a CSV separator, we are going to trim
	// the custom tag separator and use the first rune.
	if runes := []rune(strings.TrimSpace(TagSeparator)); len(runes) > 0 {
		writer.Comma = runes[0]
	}

	return writer
}

// SetCSVWriter sets the SafeCSVWriter used to format CSV.
func SetCSVWriter(csvWriter func(io.Writer) *SafeCSVWriter) {
	selfCSVWriter = csvWriter
}

func getCSVWriter(out io.Writer) *SafeCSVWriter {
	return selfCSVWriter(out)
}

// --------------------------------------------------------------------------
// CSVReader used to parse CSV

var selfCSVReader = DefaultCSVReader

// DefaultCSVReader is the default CSV reader used to parse CSV (cf. csv.NewReader)
func DefaultCSVReader(in io.Reader) CSVReader {
	return csv.NewReader(in)
}

// LazyCSVReader returns a lazy CSV reader, with LazyQuotes and TrimLeadingSpace.
func LazyCSVReader(in io.Reader) CSVReader {
	csvReader := csv.NewReader(in)
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true
	return csvReader
}

// SetCSVReader sets the CSV reader used to parse CSV.
func SetCSVReader(csvReader func(io.Reader) CSVReader) {
	selfCSVReader = csvReader
}

func getCSVReader(in io.Reader) CSVReader {
	return selfCSVReader(in)
}

// --------------------------------------------------------------------------
// Marshal functions

// MarshalFile saves the interface as CSV in the file.
func MarshalFile(in any, file *os.File) (err error) {
	return Marshal(in, file)
}

// MarshalString returns the CSV string from the interface.
func MarshalString(in any) (out string, err error) {
	bufferString := bytes.NewBufferString(out)
	if err := Marshal(in, bufferString); err != nil {
		return "", err
	}
	return bufferString.String(), nil
}

// MarshalStringWithoutHeaders returns the CSV string from the interface.
func MarshalStringWithoutHeaders(in any) (out string, err error) {
	bufferString := bytes.NewBufferString(out)
	if err := MarshalWithoutHeaders(in, bufferString); err != nil {
		return "", err
	}
	return bufferString.String(), nil
}

// MarshalBytes returns the CSV bytes from the interface.
func MarshalBytes(in any) (out []byte, err error) {
	bufferString := bytes.NewBuffer(out)
	if err := Marshal(in, bufferString); err != nil {
		return nil, err
	}
	return bufferString.Bytes(), nil
}

// MarshalBytesWithoutHeaders returns the CSV bytes from the interface.
func MarshalBytesWithoutHeaders(in any) (out []byte, err error) {
	bufferString := bytes.NewBuffer(out)
	if err := MarshalWithoutHeaders(in, bufferString); err != nil {
		return nil, err
	}
	return bufferString.Bytes(), nil
}

// Marshal returns the CSV in writer from the interface.
func Marshal(in any, out io.Writer, options ...OptionModifier) (err error) {
	writer := getCSVWriter(out)
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return writeTo(writer, in, false, modifier)
}

// MarshalWithoutHeaders returns the CSV in writer from the interface.
func MarshalWithoutHeaders(in any, out io.Writer, options ...OptionModifier) (err error) {
	writer := getCSVWriter(out)
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return writeTo(writer, in, true, modifier)
}

// MarshalChan returns the CSV read from the channel.
func MarshalChan(c <-chan any, out CSVWriter, options ...OptionModifier) error {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return writeFromChan(out, c, false, modifier)
}

// MarshalChanWithoutHeaders returns the CSV read from the channel.
func MarshalChanWithoutHeaders(c <-chan any, out CSVWriter, options ...OptionModifier) error {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return writeFromChan(out, c, true, modifier)
}

// MarshalCSV returns the CSV in writer from the interface.
func MarshalCSV(in any, out CSVWriter, options ...OptionModifier) (err error) {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return writeTo(out, in, false, modifier)
}

// MarshalCSVWithoutHeaders returns the CSV in writer from the interface.
func MarshalCSVWithoutHeaders(in any, out CSVWriter, options ...OptionModifier) (err error) {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return writeTo(out, in, true, modifier)
}

// --------------------------------------------------------------------------
// Unmarshal functions

// UnmarshalFile parses the CSV from the file in the interface.
func UnmarshalFile(in *os.File, out any, options ...OptionModifier) error {
	return Unmarshal(in, out, options...)
}

// UnmarshalMultipartFile parses the CSV from the multipart file in the interface.
func UnmarshalMultipartFile(in *multipart.File, out any, options ...OptionModifier) error {
	return Unmarshal(convertTo(in), out, options...)
}

// UnmarshalFileWithErrorHandler parses the CSV from the file in the interface.
func UnmarshalFileWithErrorHandler(in *os.File, errHandler ErrorHandler, out any, options ...OptionModifier) error {
	return UnmarshalWithErrorHandler(in, errHandler, out, options...)
}

// UnmarshalString parses the CSV from the string in the interface.
func UnmarshalString(in string, out any, options ...OptionModifier) error {
	return Unmarshal(strings.NewReader(in), out, options...)
}

// UnmarshalBytes parses the CSV from the bytes in the interface.
func UnmarshalBytes(in []byte, out any, options ...OptionModifier) error {
	return Unmarshal(bytes.NewReader(in), out, options...)
}

// Unmarshal parses the CSV from the reader in the interface.
func Unmarshal(in io.Reader, out any, options ...OptionModifier) error {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readTo(newSimpleDecoderFromReader(in), out, modifier)
}

// Unmarshal parses the CSV from the reader in the interface.
func UnmarshalWithErrorHandler(in io.Reader, errHandle ErrorHandler, out any, options ...OptionModifier) error {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readToWithErrorHandler(newSimpleDecoderFromReader(in), errHandle, out, modifier)
}

// UnmarshalWithoutHeaders parses the CSV from the reader in the interface.
func UnmarshalWithoutHeaders(in io.Reader, out any, options ...OptionModifier) error {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readToWithoutHeaders(newSimpleDecoderFromReader(in), out, modifier)
}

// UnmarshalCSVWithoutHeaders parses a headerless CSV with passed in CSV reader
func UnmarshalCSVWithoutHeaders(in CSVReader, out any, options ...OptionModifier) error {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readToWithoutHeaders(csvDecoder{in}, out, modifier)
}

// UnmarshalDecoder parses the CSV from the decoder in the interface
func UnmarshalDecoder(in Decoder, out any, options ...OptionModifier) error {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readTo(in, out, modifier)
}

// UnmarshalCSV parses the CSV from the reader in the interface.
func UnmarshalCSV(in CSVReader, out any, options ...OptionModifier) error {
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readTo(csvDecoder{in}, out, modifier)
}

// UnmarshalCSVToMap parses a CSV of 2 columns into a map.
func UnmarshalCSVToMap(in CSVReader, out any) error {
	decoder := NewSimpleDecoderFromCSVReader(in)
	header, err := decoder.GetCSVRow()
	if err != nil {
		return err
	}
	if len(header) != 2 {
		return fmt.Errorf("maps can only be created for csv of two columns")
	}
	outValue, outType := getConcreteReflectValueAndType(out)
	if outType.Kind() != reflect.Map {
		return fmt.Errorf("cannot use %s, only map supported", outType.String())
	}
	keyType := outType.Key()
	valueType := outType.Elem()
	outValue.Set(reflect.MakeMap(outType))
	for {
		key := reflect.New(keyType)
		value := reflect.New(valueType)
		line, err := decoder.GetCSVRow()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if err := setField(key, line[0], false); err != nil {
			return err
		}
		if err := setField(value, line[1], false); err != nil {
			return err
		}
		outValue.SetMapIndex(key.Elem(), value.Elem())
	}
	return nil
}

// UnmarshalToChan parses the CSV from the reader and send each value in the chan c.
// The channel must have a concrete type.
func UnmarshalToChan(in io.Reader, c any, options ...OptionModifier) error {
	if c == nil {
		return fmt.Errorf("goscv: channel is %v", c)
	}
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readEach(newSimpleDecoderFromReader(in), nil, c, modifier)
}

// UnmarshalToChanWithErrorHandler parses the CSV from the reader in the interface.
func UnmarshalToChanWithErrorHandler(in io.Reader, errorHandler ErrorHandler, c any, options ...OptionModifier) error {
	if c == nil {
		return fmt.Errorf("goscv: channel is %v", c)
	}
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readEach(newSimpleDecoderFromReader(in), errorHandler, c, modifier)
}

// UnmarshalToChanWithoutHeaders parses the CSV from the reader and send each value in the chan c.
// The channel must have a concrete type.
func UnmarshalToChanWithoutHeaders(in io.Reader, c any, options ...OptionModifier) error {
	if c == nil {
		return fmt.Errorf("goscv: channel is %v", c)
	}

	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readEachWithoutHeaders(newSimpleDecoderFromReader(in), c, modifier)
}

// UnmarshalDecoderToChan parses the CSV from the decoder and send each value in the chan c.
// The channel must have a concrete type.
func UnmarshalDecoderToChan(in SimpleDecoder, c any, options ...OptionModifier) error {
	if c == nil {
		return fmt.Errorf("goscv: channel is %v", c)
	}
	modifier := DefaultModifier()
	for _, option := range options {
		option(modifier)
	}
	return readEach(in, nil, c, modifier)
}

// UnmarshalStringToChan parses the CSV from the string and send each value in the chan c.
// The channel must have a concrete type.
func UnmarshalStringToChan(in string, c any) error {
	return UnmarshalToChan(strings.NewReader(in), c)
}

// UnmarshalBytesToChan parses the CSV from the bytes and send each value in the chan c.
// The channel must have a concrete type.
func UnmarshalBytesToChan(in []byte, c any) error {
	return UnmarshalToChan(bytes.NewReader(in), c)
}

// UnmarshalToCallback parses the CSV from the reader and send each value to the given func f.
// The func must look like func(Struct). If its last return value is an error,
// the parser is drained without further callback calls before returning that error.
func UnmarshalToCallback(in io.Reader, f any) error {
	valueFunc := reflect.ValueOf(f)
	t := reflect.TypeOf(f)
	if t.NumIn() != 1 {
		return fmt.Errorf("the given function must have exactly one parameter")
	}
	cerr := make(chan error)
	c := reflect.MakeChan(reflect.ChanOf(reflect.BothDir, t.In(0)), 0)
	go func() {
		cerr <- UnmarshalToChan(in, c.Interface())
	}()
	var callbackErr error
	for {
		select {
		case err := <-cerr:
			if callbackErr != nil {
				return callbackErr
			}
			return err
		default:
		}
		v, notClosed := c.Recv()
		if !notClosed || v.Interface() == nil {
			break
		}
		if callbackErr != nil {
			continue // Drain the producer without invoking the callback again.
		}
		callResults := valueFunc.Call([]reflect.Value{v})
		// Preserve a callback error while allowing the producer to finish.
		if len(callResults) > 0 {
			if err, ok := callResults[len(callResults)-1].Interface().(error); ok {
				callbackErr = err
			}
		}
	}
	err := <-cerr
	if callbackErr != nil {
		return callbackErr
	}
	return err
}

// UnmarshalDecoderToCallback parses the CSV from the decoder and send each value to the given func f.
// The func must look like func(Struct).
func UnmarshalDecoderToCallback(in SimpleDecoder, f any) error {
	valueFunc := reflect.ValueOf(f)
	t := reflect.TypeOf(f)
	if t.NumIn() != 1 {
		return fmt.Errorf("the given function must have exactly one parameter")
	}
	cerr := make(chan error)
	c := reflect.MakeChan(reflect.ChanOf(reflect.BothDir, t.In(0)), 0)
	go func() {
		cerr <- UnmarshalDecoderToChan(in, c.Interface())
	}()
	for {
		select {
		case err := <-cerr:
			return err
		default:
		}
		v, notClosed := c.Recv()
		if !notClosed || v.Interface() == nil {
			break
		}
		valueFunc.Call([]reflect.Value{v})
	}
	return <-cerr
}

// UnmarshalBytesToCallback parses the CSV from the bytes and send each value to the given func f.
// The func must look like func(Struct).
func UnmarshalBytesToCallback(in []byte, f any) error {
	return UnmarshalToCallback(bytes.NewReader(in), f)
}

// UnmarshalStringToCallback parses the CSV from the string and send each value to the given func f.
// The func must look like func(Struct).
func UnmarshalStringToCallback(in string, c any) (err error) {
	return UnmarshalToCallback(strings.NewReader(in), c)
}

// UnmarshalToCallbackWithError parses the CSV from the reader and
// send each value to the given func f.
//
// If func returns error, it will stop processing, drain the
// parser and propagate the error to caller.
//
// The func must look like func(Struct) error.
func UnmarshalToCallbackWithError(in io.Reader, f any) error {
	valueFunc := reflect.ValueOf(f)
	t := reflect.TypeOf(f)
	if t.NumIn() != 1 {
		return fmt.Errorf("the given function must have exactly one parameter")
	}
	if t.NumOut() != 1 {
		return fmt.Errorf("the given function must have exactly one return value")
	}
	if !isErrorType(t.Out(0)) {
		return fmt.Errorf("the given function must only return error")
	}

	cerr := make(chan error)
	c := reflect.MakeChan(reflect.ChanOf(reflect.BothDir, t.In(0)), 0)
	go func() {
		cerr <- UnmarshalToChan(in, c.Interface())
	}()

	var fErr error
	for {
		select {
		case err := <-cerr:
			if err != nil {
				return err
			}
			return fErr
		default:
		}
		v, notClosed := c.Recv()
		if !notClosed || v.Interface() == nil {
			if err := <-cerr; err != nil {
				fErr = err
			}
			break
		}

		// callback f has already returned an error, stop processing but keep draining the chan c
		if fErr != nil {
			continue
		}

		results := valueFunc.Call([]reflect.Value{v})

		// If the callback f returns an error, stores it and returns it in future.
		errValue := results[0].Interface()
		if errValue != nil {
			fErr = errValue.(error)
		}
	}
	return fErr
}

// UnmarshalBytesToCallbackWithError parses the CSV from the bytes and
// send each value to the given func f.
//
// If func returns error, it will stop processing, drain the
// parser and propagate the error to caller.
//
// The func must look like func(Struct) error.
func UnmarshalBytesToCallbackWithError(in []byte, f any) error {
	return UnmarshalToCallbackWithError(bytes.NewReader(in), f)
}

// UnmarshalStringToCallbackWithError parses the CSV from the string and
// send each value to the given func f.
//
// If func returns error, it will stop processing, drain the
// parser and propagate the error to caller.
//
// The func must look like func(Struct) error.
func UnmarshalStringToCallbackWithError(in string, c any) (err error) {
	return UnmarshalToCallbackWithError(strings.NewReader(in), c)
}

// CSVToMap creates a simple map from a CSV of 2 columns.
func CSVToMap(in io.Reader) (map[string]string, error) {
	decoder := newSimpleDecoderFromReader(in)
	header, err := decoder.GetCSVRow()
	if err != nil {
		return nil, err
	}
	if len(header) != 2 {
		return nil, fmt.Errorf("maps can only be created for csv of two columns")
	}
	m := make(map[string]string)
	for {
		line, err := decoder.GetCSVRow()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		m[line[0]] = line[1]
	}
	return m, nil
}

// CSVToMaps takes a reader and returns an array of dictionaries, using the header row as the keys
func CSVToMaps(reader io.Reader) ([]map[string]string, error) {
	r := getCSVReader(reader)
	rows := []map[string]string{}
	var header []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header == nil {
			header = record
		} else {
			dict := map[string]string{}
			for i := range header {
				dict[header[i]] = record[i]
			}
			rows = append(rows, dict)
		}
	}
	return rows, nil
}

// CSVToChanMaps parses the CSV from the reader and send a dictionary in the chan c, using the header row as the keys.
func CSVToChanMaps(reader io.Reader, c chan<- map[string]string) error {
	r := csv.NewReader(reader)
	var header []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header == nil {
			header = record
		} else {
			dict := map[string]string{}
			for i := range header {
				dict[header[i]] = record[i]
			}
			c <- dict
		}
	}
	return nil
}
