package gocsv

import (
	"bytes"
	"testing"
)

type fieldsCombinerTestInner struct {
	Value string `csv:"value"`
}

type fieldsCombinerTestRow struct {
	Inner fieldsCombinerTestInner `csv:"inner"`
}

func TestFieldsCombinerIsAppliedPerCall(t *testing.T) {
	rows := []fieldsCombinerTestRow{
		{Inner: fieldsCombinerTestInner{Value: "one"}},
	}

	tests := []struct {
		name    string
		options []OptionModifier
		want    string
	}{
		{
			name: "default separator",
			want: "inner.value\none\n",
		},
		{
			name: "custom separator",
			options: []OptionModifier{
				WithFieldsCombiner("_"),
			},
			want: "inner_value\none\n",
		},
		{
			name: "default separator after custom separator",
			want: "inner.value\none\n",
		},
	}

	t.Run("Marshal", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var output bytes.Buffer

				err := Marshal(rows, &output, tt.options...)
				if err != nil {
					t.Fatal(err)
				}

				if got := output.String(); got != tt.want {
					t.Errorf("Marshal() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("Unmarshal", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var got []fieldsCombinerTestRow

				if err := UnmarshalString(
					tt.want,
					&got,
					tt.options...,
				); err != nil {
					t.Fatal(err)
				}

				if len(got) != 1 {
					t.Fatalf("got %d rows, want 1", len(got))
				}

				if got[0].Inner.Value != "one" {
					t.Errorf(
						"Inner.Value = %q, want %q",
						got[0].Inner.Value,
						"one",
					)
				}
			})
		}
	})
}
