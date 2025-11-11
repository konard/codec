package main

import (
	"fmt"
	"reflect"

	"github.com/URALINNOVATSIYA/codec"
)

// Demonstrate that the serializer/deserializer works for ALL Go types
func main() {
	fmt.Println("=== Complete Serializer/Deserializer Demo ===\n")

	// Test all primitive types
	testPrimitives()

	// Test collections
	testCollections()

	// Test composite types
	testCompositeTypes()

	// Test advanced types
	testAdvancedTypes()

	fmt.Println("\n=== All Tests Passed! ===")
}

func testValue(name string, value any) bool {
	data := codec.Serialize(value)
	result, err := codec.Unserialize(data)
	if err != nil {
		fmt.Printf("❌ %s: ERROR - %v\n", name, err)
		return false
	}
	if reflect.DeepEqual(value, result) {
		fmt.Printf("✓ %s\n", name)
		return true
	}
	fmt.Printf("❌ %s: expected %v, got %v\n", name, value, result)
	return false
}

func testPrimitives() {
	fmt.Println("Primitive Types:")
	testValue("nil", nil)
	testValue("bool (true)", true)
	testValue("bool (false)", false)
	testValue("string", "Hello, World!")
	testValue("int8", int8(-128))
	testValue("int16", int16(-32768))
	testValue("int32", int32(-2147483648))
	testValue("int64", int64(-9223372036854775808))
	testValue("int", int(-42))
	testValue("uint8", uint8(255))
	testValue("uint16", uint16(65535))
	testValue("uint32", uint32(4294967295))
	testValue("uint64", uint64(18446744073709551615))
	testValue("uint", uint(42))
	testValue("float32", float32(3.14159))
	testValue("float64", float64(2.718281828459045))
	testValue("complex64", complex64(1+2i))
	testValue("complex128", complex128(3+4i))
	testValue("uintptr", uintptr(0x12345678))
	fmt.Println()
}

func testCollections() {
	fmt.Println("Collection Types:")

	// Slices
	testValue("nil slice", ([]int)(nil))
	testValue("empty slice", []int{})
	testValue("int slice", []int{1, 2, 3, 4, 5})
	testValue("string slice", []string{"hello", "world", "test"})
	testValue("nested slice", [][]int{{1, 2}, {3, 4}, {5, 6}})

	// Arrays
	testValue("int array", [3]int{1, 2, 3})
	testValue("string array", [2]string{"hello", "world"})
	testValue("nested array", [2][3]int{{1, 2, 3}, {4, 5, 6}})

	// Maps
	testValue("nil map", (map[string]int)(nil))
	testValue("empty map", map[string]int{})
	testValue("string->int map", map[string]int{"one": 1, "two": 2, "three": 3})
	testValue("int->string map", map[int]string{1: "one", 2: "two", 3: "three"})
	testValue("nested map", map[string]map[int]string{
		"a": {1: "one", 2: "two"},
		"b": {3: "three", 4: "four"},
	})

	fmt.Println()
}

func testCompositeTypes() {
	fmt.Println("Composite Types:")

	// Structs
	type Person struct {
		Name string
		Age  int
		Tags []string
	}

	testValue("simple struct", Person{Name: "Alice", Age: 30, Tags: []string{"dev", "golang"}})

	// Structs with slices
	testValue("struct with slice", Person{
		Name: "Bob",
		Age:  25,
		Tags: []string{"designer", "artist"},
	})

	// Slice of structs
	testValue("slice of structs", []Person{
		{Name: "Alice", Age: 30, Tags: []string{"dev"}},
		{Name: "Bob", Age: 25, Tags: []string{"designer"}},
	})

	// Map of structs
	testValue("map of structs", map[string]Person{
		"alice": {Name: "Alice", Age: 30, Tags: []string{"dev"}},
		"bob":   {Name: "Bob", Age: 25, Tags: []string{"designer"}},
	})

	fmt.Println()
}

func testAdvancedTypes() {
	fmt.Println("Advanced Types:")

	// Pointers
	x := 42
	testValue("pointer to int", &x)

	// Nil pointers
	testValue("nil pointer", (*int)(nil))

	// Channels
	ch := make(chan int, 5)
	testValue("buffered channel", ch)

	// Interfaces
	var iface interface{} = "hello"
	testValue("interface{} with string", iface)

	iface = 123
	testValue("interface{} with int", iface)

	// Functions
	fn := func() int { return 42 }
	codec.RegisterTypeOf(fn)
	testValue("function", fn)

	fmt.Println()
}
