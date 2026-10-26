package gocsv

type Modifiers struct {
	// fieldsCombiner defines how to combine parent struct and child struct
	fieldsCombiner string
	// failIfUnmatchedStructTags indicates whether it is considered an error when there is an unmatched
	// struct tag.
	failIfUnmatchedStructTags bool
	// failIfDoubleHeaderNames indicates whether it is considered an error when a header name is repeated
	// in the csv header.
	failIfDoubleHeaderNames bool
	// shouldAlignDuplicateHeadersWithStructFieldOrder indicates whether we should align duplicate CSV
	// headers per their alignment in the struct definition.
	shouldAlignDuplicateHeadersWithStructFieldOrder bool
}

func DefaultModifier() *Modifiers {
	return &Modifiers{
		fieldsCombiner:                                  ".",
		failIfUnmatchedStructTags:                       false,
		failIfDoubleHeaderNames:                         false,
		shouldAlignDuplicateHeadersWithStructFieldOrder: false,
	}
}

type OptionModifier func(*Modifiers) *Modifiers

// WithFieldsCombiner defaults to "."
// defines how to combine parent struct and child struct
func WithFieldsCombiner(fieldsCombiner string) OptionModifier {
	return func(o *Modifiers) *Modifiers {
		o.fieldsCombiner = fieldsCombiner
		return o
	}
}

// WithUnmatchTagFailure defaults to false
// determines whether it is considered an error when there is an unmatched struct tag
func WithUnmatchTagFailure(failOnUnmatachedTag bool) OptionModifier {
	return func(o *Modifiers) *Modifiers {
		o.failIfUnmatchedStructTags = failOnUnmatachedTag
		return o
	}
}

// WithDuplicateHeaderFailure defaults to false
// failIfDoubleHeaderNames indicates whether it is considered an error when a header name is repeated
// in the csv header
func WithDuplicateHeaderFailure(failOnDuplicateHeaders bool) OptionModifier {
	return func(o *Modifiers) *Modifiers {
		o.failIfDoubleHeaderNames = failOnDuplicateHeaders
		return o
	}
}

// WithDuplicateHeader defaults to false
// shouldAlignDuplicateHeadersWithStructFieldOrder indicates whether we should align duplicate CSV
// headers per their alignment in the struct definition.
func WithAlignDuplicateHeader(alignDuplicateHeaders bool) OptionModifier {
	return func(o *Modifiers) *Modifiers {
		o.shouldAlignDuplicateHeadersWithStructFieldOrder = alignDuplicateHeaders
		return o
	}
}
