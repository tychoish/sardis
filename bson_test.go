package sardis

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/tychoish/birch"
	"go.mongodb.org/mongo-driver/bson"
)

func TestInterBSON(t *testing.T) {
	input := map[string]string{}
	for i := 0; i < 100; i++ {
		input[fmt.Sprint("key", i)] = fmt.Sprint("value", i*2)
	}

	output, err := bson.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	boutput, err := birch.DC.MapString(input).MarshalBSON()

	t.Log(bytes.Equal(boutput, output))
	t.Log(output)
	t.Log(boutput)
	rt := map[string]string{}
	err = bson.Unmarshal(boutput, &rt)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(input)
	t.Log(rt)
}

func BenchmarkBSON(b *testing.B) {
	input := map[string]string{}
	for i := 0; i < 100; i++ {
		input[fmt.Sprint("key", i)] = fmt.Sprint("value", i*2)
	}

	output, err := bson.Marshal(input)
	b.Run("Marshaling", func(b *testing.B) {
		b.Run("Reference", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				output, err = bson.Marshal(input)
				if err != nil || len(output) == 0 {
					b.Fatal()
				}
			}
		})
		b.Run("BirchConstructor", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				doc := birch.DC.MapString(input)
				if doc.Len() != 100 {
					b.Fatal()
				}
			}
		})
		b.Run("BirchMarshaler", func(b *testing.B) {
			doc := birch.DC.MapString(input)
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				output, err = doc.MarshalBSON()
				if err != nil || len(output) == 0 {
					b.Fatal()
				}
			}
		})
		b.Run("BirchCombined", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				output, err = birch.DC.MapString(input).MarshalBSON()
				if err != nil || len(output) == 0 {
					b.Fatal()
				}
			}
		})
	})
	b.Run("Unmarshaling", func(b *testing.B) {
		b.Run("Reference", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resolved := map[string]string{}
				err = bson.Unmarshal(output, &resolved)
				if err != nil || len(resolved) != 100 {
					b.Fatal(err, len(resolved))
				}
			}
		})
		b.Run("BirchExport", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resolved := birch.DC.Reader(output).ExportMap()
				if err != nil || len(resolved) != 100 {
					b.Fatal(err, len(resolved))
				}
			}
		})
		b.Run("BirchNoExport", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resolved := birch.DC.ReadFrom(bytes.NewBuffer(output))
				if err != nil || resolved.Len() != 100 {
					b.Fatal(err, resolved.Len())
				}
			}
		})
		b.Run("BirchPrealloc", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resolved := birch.DC.Make(100)
				err := resolved.UnmarshalBSON(output)
				if err != nil || resolved.Len() != 100 {
					b.Fatal(err, resolved.Len())
				}
			}
		})
		b.Run("BirchDirect", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resolved := birch.DC.New()
				err := resolved.UnmarshalBSON(output)
				if err != nil || resolved.Len() != 100 {
					b.Fatal(err, resolved.Len())
				}
			}
		})

	})

}
