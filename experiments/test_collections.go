package main

import (
	"fmt"
	"reflect"

	"github.com/URALINNOVATSIYA/codec"
)

func testValue(name string, value any) {
	data := codec.Serialize(value)
	fmt.Printf("%s: serialized %d bytes\n", name, len(data))

	result, err := codec.Unserialize(data)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}

	if reflect.DeepEqual(value, result) {
		fmt.Printf("  ✓ Success\n")
	} else {
		fmt.Printf("  ✗ FAIL: expected %v, got %v\n", value, result)
	}
}

func main() {
	fmt.Println("Testing Slices:")
	testValue("nil slice", ([]int)(nil))
	testValue("empty slice", []int{})
	testValue("int slice", []int{1, 2, 3, 4, 5})
	testValue("string slice", []string{"hello", "world"})
	testValue("byte slice", []byte{1, 2, 3})
	testValue("nested slice", [][]int{{1, 2}, {3, 4}, {5, 6}})

	fmt.Println("\nTesting Arrays:")
	testValue("int array", [3]int{1, 2, 3})
	testValue("string array", [2]string{"hello", "world"})
	testValue("byte array", [5]byte{1, 2, 3, 4, 5})
	testValue("nested array", [2][3]int{{1, 2, 3}, {4, 5, 6}})

	fmt.Println("\nTesting Maps:")
	testValue("nil map", (map[string]int)(nil))
	testValue("empty map", map[string]int{})
	testValue("string->int map", map[string]int{"one": 1, "two": 2, "three": 3})
	testValue("int->string map", map[int]string{1: "one", 2: "two", 3: "three"})
	testValue("nested map", map[string]map[int]string{"a": {1: "one"}, "b": {2: "two"}})

	fmt.Println("\nTesting Complex Structures:")
	type Person struct {
		Name string
		Age  int
		Tags []string
	}
	testValue("struct with slice", Person{Name: "Alice", Age: 30, Tags: []string{"developer", "golang"}})

	testValue("slice of structs", []Person{
		{Name: "Alice", Age: 30, Tags: []string{"developer"}},
		{Name: "Bob", Age: 25, Tags: []string{"designer"}},
	})

	testValue("map of structs", map[string]Person{
		"alice": {Name: "Alice", Age: 30, Tags: []string{"developer"}},
		"bob":   {Name: "Bob", Age: 25, Tags: []string{"designer"}},
	})
}
