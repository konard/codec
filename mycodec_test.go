package main

import (
	"reflect"
	"testing"
)

// Test primitives
func TestPrimitives(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"nil", nil},
		{"bool_true", true},
		{"bool_false", false},
		{"int", int(42)},
		{"int8", int8(-128)},
		{"int16", int16(1000)},
		{"int32", int32(100000)},
		{"int64", int64(9223372036854775807)},
		{"uint", uint(42)},
		{"uint8", uint8(255)},
		{"uint16", uint16(65535)},
		{"uint32", uint32(4294967295)},
		{"uint64", uint64(18446744073709551615)},
		{"uintptr", uintptr(12345)},
		{"float32", float32(3.14)},
		{"float64", float64(2.718281828)},
		{"complex64", complex64(1 + 2i)},
		{"complex128", complex128(3.14 + 2.71i)},
		{"string", "Hello, 世界!"},
		{"empty_string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if !reflect.DeepEqual(tt.value, result) {
				t.Errorf("Value mismatch: expected %v (%T), got %v (%T)",
					tt.value, tt.value, result, result)
			}
		})
	}
}

// Test slices
func TestSlices(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"nil_slice", []int(nil)},
		{"empty_slice", []int{}},
		{"int_slice", []int{1, 2, 3, 4, 5}},
		{"string_slice", []string{"hello", "world"}},
		{"byte_slice", []byte{1, 2, 3}},
		{"nested_slice", [][]int{{1, 2}, {3, 4}, {5, 6}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if !reflect.DeepEqual(tt.value, result) {
				t.Errorf("Value mismatch: expected %v, got %v", tt.value, result)
			}
		})
	}
}

// Test arrays
func TestArrays(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"int_array", [3]int{1, 2, 3}},
		{"string_array", [2]string{"hello", "world"}},
		{"bool_array", [4]bool{true, false, true, false}},
		{"nested_array", [2][3]int{{1, 2, 3}, {4, 5, 6}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if !reflect.DeepEqual(tt.value, result) {
				t.Errorf("Value mismatch: expected %v, got %v", tt.value, result)
			}
		})
	}
}

// Test maps
func TestMaps(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"nil_map", map[string]int(nil)},
		{"empty_map", map[string]int{}},
		{"string_int_map", map[string]int{"one": 1, "two": 2, "three": 3}},
		{"int_string_map", map[int]string{1: "one", 2: "two", 3: "three"}},
		{"nested_map", map[string]map[string]int{
			"first":  {"a": 1, "b": 2},
			"second": {"c": 3, "d": 4},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if !reflect.DeepEqual(tt.value, result) {
				t.Errorf("Value mismatch: expected %v, got %v", tt.value, result)
			}
		})
	}
}

// Test structs
func TestStructs(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	type Company struct {
		Name      string
		Employees []Person
	}

	tests := []struct {
		name  string
		value interface{}
	}{
		{"simple_struct", Person{Name: "Alice", Age: 30}},
		{"nested_struct", Company{
			Name: "TechCorp",
			Employees: []Person{
				{Name: "Alice", Age: 30},
				{Name: "Bob", Age: 25},
			},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if !reflect.DeepEqual(tt.value, result) {
				t.Errorf("Value mismatch: expected %v, got %v", tt.value, result)
			}
		})
	}
}

// Test pointers
func TestPointers(t *testing.T) {
	intVal := 42
	strVal := "hello"

	tests := []struct {
		name  string
		value interface{}
	}{
		{"nil_pointer", (*int)(nil)},
		{"int_pointer", &intVal},
		{"string_pointer", &strVal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if !reflect.DeepEqual(tt.value, result) {
				t.Errorf("Value mismatch: expected %v, got %v", tt.value, result)
			}
		})
	}
}

// Test cyclic pointers
func TestCyclicPointers(t *testing.T) {
	type Node struct {
		Value int
		Next  *Node
	}

	// Create a cycle: n1 -> n2 -> n3 -> n1
	n1 := &Node{Value: 1}
	n2 := &Node{Value: 2}
	n3 := &Node{Value: 3}
	n1.Next = n2
	n2.Next = n3
	n3.Next = n1

	data, err := Serialize(n1)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	result, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	resultNode := result.(*Node)

	// Check values
	if resultNode.Value != 1 {
		t.Errorf("Expected value 1, got %d", resultNode.Value)
	}
	if resultNode.Next.Value != 2 {
		t.Errorf("Expected next value 2, got %d", resultNode.Next.Value)
	}
	if resultNode.Next.Next.Value != 3 {
		t.Errorf("Expected next.next value 3, got %d", resultNode.Next.Next.Value)
	}

	// Check that cycle is preserved
	if resultNode.Next.Next.Next != resultNode {
		t.Errorf("Cyclic reference not preserved")
	}
}

// Test interfaces
func TestInterfaces(t *testing.T) {
	var i interface{} = 42
	var s interface{} = "hello"
	var n interface{} = nil

	tests := []struct {
		name  string
		value interface{}
	}{
		{"int_interface", i},
		{"string_interface", s},
		{"nil_interface", n},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if !reflect.DeepEqual(tt.value, result) {
				t.Errorf("Value mismatch: expected %v (%T), got %v (%T)",
					tt.value, tt.value, result, result)
			}
		})
	}
}

// Test channels
func TestChannels(t *testing.T) {
	ch1 := make(chan int)
	ch2 := make(chan int, 10)
	var ch3 chan int = nil

	tests := []struct {
		name  string
		value interface{}
	}{
		{"unbuffered_chan", ch1},
		{"buffered_chan", ch2},
		{"nil_chan", ch3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			// Check type
			if reflect.TypeOf(tt.value) != reflect.TypeOf(result) {
				t.Errorf("Type mismatch: expected %T, got %T", tt.value, result)
			}

			// Check capacity for non-nil channels
			if tt.value != nil && result != nil {
				vVal := reflect.ValueOf(tt.value)
				rVal := reflect.ValueOf(result)
				if vVal.Cap() != rVal.Cap() {
					t.Errorf("Capacity mismatch: expected %d, got %d",
						vVal.Cap(), rVal.Cap())
				}
			}
		})
	}
}

// Test functions
func TestFunctions(t *testing.T) {
	fn1 := func(x int) int { return x * 2 }
	var fn2 func() = nil

	tests := []struct {
		name  string
		value interface{}
	}{
		{"non_nil_func", fn1},
		{"nil_func", fn2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Serialize(tt.value)
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			result, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			// Check type
			if reflect.TypeOf(tt.value) != reflect.TypeOf(result) {
				t.Errorf("Type mismatch: expected %T, got %T", tt.value, result)
			}

			// Functions can't be meaningfully compared, just check they deserialize
			// with correct type
		})
	}
}

// Test complex nested structures
func TestComplexStructure(t *testing.T) {
	type Address struct {
		Street string
		City   string
	}

	type Person struct {
		Name      string
		Age       int
		Addresses []Address
		Metadata  map[string]interface{}
	}

	value := Person{
		Name: "Alice",
		Age:  30,
		Addresses: []Address{
			{Street: "123 Main St", City: "NYC"},
			{Street: "456 Oak Ave", City: "LA"},
		},
		Metadata: map[string]interface{}{
			"active": true,
			"score":  95.5,
			"tags":   []string{"vip", "premium"},
		},
	}

	data, err := Serialize(value)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	result, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if !reflect.DeepEqual(value, result) {
		t.Errorf("Value mismatch: expected %+v, got %+v", value, result)
	}
}
