package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

// Test if current implementation preserves memory structures

func testPointerToSliceElement() bool {
	fmt.Println("\n=== Test: Pointer to Slice Element ===")

	type Data struct {
		Slice []int
		Ptr   *int
	}

	original := Data{
		Slice: []int{1, 2, 3, 4, 5},
	}
	original.Ptr = &original.Slice[2] // Pointer to element at index 2

	fmt.Printf("Original:\n")
	fmt.Printf("  Slice: %v\n", original.Slice)
	fmt.Printf("  *Ptr: %d\n", *original.Ptr)
	fmt.Printf("  Ptr == &Slice[2]: %v\n", original.Ptr == &original.Slice[2])

	// Serialize and deserialize
	data, err := Serialize(original)
	if err != nil {
		fmt.Printf("  ✗ Serialize error: %v\n", err)
		return false
	}

	resultInterface, err := Deserialize(data)
	if err != nil {
		fmt.Printf("  ✗ Deserialize error: %v\n", err)
		return false
	}

	// Use reflection since reflect.StructOf creates a different type
	rv := reflect.ValueOf(resultInterface)
	if rv.Kind() != reflect.Struct {
		fmt.Printf("  ✗ Expected struct, got %v\n", rv.Kind())
		return false
	}

	sliceField := rv.FieldByName("Slice")
	ptrField := rv.FieldByName("Ptr")

	if !sliceField.IsValid() || !ptrField.IsValid() {
		fmt.Printf("  ✗ Missing fields in deserialized struct\n")
		return false
	}

	fmt.Printf("\nDeserialized:\n")
	slice := sliceField.Interface().([]int)
	ptr := ptrField.Interface().(*int)
	fmt.Printf("  Slice: %v\n", slice)
	fmt.Printf("  *Ptr: %d\n", *ptr)

	// The critical test: check if pointer points to slice element
	ptrAddr := uintptr(unsafe.Pointer(ptr))
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&slice))
	elemSize := unsafe.Sizeof(int(0))
	elem2Addr := sliceHeader.Data + uintptr(2)*elemSize

	fmt.Printf("  Ptr address: %x\n", ptrAddr)
	fmt.Printf("  Slice[2] address: %x\n", elem2Addr)
	fmt.Printf("  Ptr == &Slice[2]: %v\n", ptrAddr == elem2Addr)

	if ptrAddr != elem2Addr {
		fmt.Printf("  ✗ FAIL: Pointer does NOT point to slice element\n")
		return false
	}

	// Verify modification propagates
	*ptr = 100
	sliceField2 := rv.FieldByName("Slice")
	slice2 := sliceField2.Interface().([]int)
	if slice2[2] != 100 {
		fmt.Printf("  ✗ FAIL: Modification through pointer did not update slice\n")
		return false
	}

	fmt.Printf("  ✓ PASS: Memory structure preserved\n")
	return true
}

func testSharedSliceBackingArray() bool {
	fmt.Println("\n=== Test: Shared Slice Backing Array ===")

	type Data struct {
		Slice1 []int
		Slice2 []int
	}

	backing := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	original := Data{
		Slice1: backing[2:5], // [3, 4, 5]
		Slice2: backing[5:8], // [6, 7, 8]
	}

	fmt.Printf("Original:\n")
	fmt.Printf("  Slice1: %v\n", original.Slice1)
	fmt.Printf("  Slice2: %v\n", original.Slice2)

	sh1 := (*reflect.SliceHeader)(unsafe.Pointer(&original.Slice1))
	sh2 := (*reflect.SliceHeader)(unsafe.Pointer(&original.Slice2))
	fmt.Printf("  Data pointers differ by: %d bytes\n", int(sh2.Data)-int(sh1.Data))

	// Serialize and deserialize
	data, err := Serialize(original)
	if err != nil {
		fmt.Printf("  ✗ Serialize error: %v\n", err)
		return false
	}

	resultInterface, err := Deserialize(data)
	if err != nil {
		fmt.Printf("  ✗ Deserialize error: %v\n", err)
		return false
	}

	// Use reflection
	rv := reflect.ValueOf(resultInterface)
	if rv.Kind() != reflect.Struct {
		fmt.Printf("  ✗ Expected struct, got %v\n", rv.Kind())
		return false
	}

	slice1Field := rv.FieldByName("Slice1")
	slice2Field := rv.FieldByName("Slice2")

	if !slice1Field.IsValid() || !slice2Field.IsValid() {
		fmt.Printf("  ✗ Missing fields in deserialized struct\n")
		return false
	}

	slice1 := slice1Field.Interface().([]int)
	slice2 := slice2Field.Interface().([]int)

	fmt.Printf("\nDeserialized:\n")
	fmt.Printf("  Slice1: %v\n", slice1)
	fmt.Printf("  Slice2: %v\n", slice2)

	rsh1 := (*reflect.SliceHeader)(unsafe.Pointer(&slice1))
	rsh2 := (*reflect.SliceHeader)(unsafe.Pointer(&slice2))
	fmt.Printf("  Data pointers differ by: %d bytes\n", int(rsh2.Data)-int(rsh1.Data))

	// The critical test: slices should share backing array
	// They should be offset by the same amount (3 ints = 24 bytes on 64-bit)
	origDiff := int(sh2.Data) - int(sh1.Data)
	resDiff := int(rsh2.Data) - int(rsh1.Data)

	if origDiff != resDiff {
		fmt.Printf("  ✗ FAIL: Backing array not shared (offset changed from %d to %d)\n", origDiff, resDiff)
		return false
	}

	fmt.Printf("  ✓ PASS: Memory structure preserved\n")
	return true
}

func testPointerChain() bool {
	fmt.Println("\n=== Test: Pointer Chain ===")

	value := 42
	ptr1 := &value
	ptr2 := &ptr1
	ptr3 := &ptr2

	original := ptr3

	fmt.Printf("Original: ptr3 -> ptr2 -> ptr1 -> value\n")
	fmt.Printf("  ***ptr3 = %d\n", ***original)

	// Serialize and deserialize
	data, err := Serialize(original)
	if err != nil {
		fmt.Printf("  ✗ Serialize error: %v\n", err)
		return false
	}

	resultInterface, err := Deserialize(data)
	if err != nil {
		fmt.Printf("  ✗ Deserialize error: %v\n", err)
		return false
	}

	// Type assertion to ***int
	result, ok := resultInterface.(***int)
	if !ok {
		fmt.Printf("  ✗ Type assertion failed: got %T\n", resultInterface)
		return false
	}

	fmt.Printf("\nDeserialized:\n")
	fmt.Printf("  ***result = %d\n", ***result)

	// Verify modification propagates through all levels
	***result = 100
	if ***result != 100 {
		fmt.Printf("  ✗ FAIL: Modification did not propagate\n")
		return false
	}

	// Verify all levels point to the same value
	val := ***result
	ptr1_result := **result
	ptr2_result := *result

	if *ptr1_result != val || **ptr2_result != val {
		fmt.Printf("  ✗ FAIL: Pointer chain broken\n")
		return false
	}

	fmt.Printf("  ✓ PASS: Memory structure preserved\n")
	return true
}

func testMultipleReferencesToSameSlice() bool {
	fmt.Println("\n=== Test: Multiple References to Same Slice ===")

	type Data struct {
		Slice1 []int
		Slice2 []int
		Ptr    *[]int
	}

	slice := []int{1, 2, 3, 4, 5}
	original := Data{
		Slice1: slice,
		Slice2: slice,
		Ptr:    &slice,
	}

	fmt.Printf("Original:\n")
	sh1 := (*reflect.SliceHeader)(unsafe.Pointer(&original.Slice1))
	sh2 := (*reflect.SliceHeader)(unsafe.Pointer(&original.Slice2))
	sh3 := (*reflect.SliceHeader)(unsafe.Pointer(original.Ptr))
	fmt.Printf("  All data pointers equal: %v\n", sh1.Data == sh2.Data && sh2.Data == sh3.Data)

	// Serialize and deserialize
	data, err := Serialize(original)
	if err != nil {
		fmt.Printf("  ✗ Serialize error: %v\n", err)
		return false
	}

	resultInterface, err := Deserialize(data)
	if err != nil {
		fmt.Printf("  ✗ Deserialize error: %v\n", err)
		return false
	}

	// Use reflection
	rv := reflect.ValueOf(resultInterface)
	if rv.Kind() != reflect.Struct {
		fmt.Printf("  ✗ Expected struct, got %v\n", rv.Kind())
		return false
	}

	slice1Field := rv.FieldByName("Slice1")
	slice2Field := rv.FieldByName("Slice2")
	ptrField := rv.FieldByName("Ptr")

	if !slice1Field.IsValid() || !slice2Field.IsValid() || !ptrField.IsValid() {
		fmt.Printf("  ✗ Missing fields in deserialized struct\n")
		return false
	}

	slice1 := slice1Field.Interface().([]int)
	slice2 := slice2Field.Interface().([]int)
	ptr := ptrField.Interface().(*[]int)

	fmt.Printf("\nDeserialized:\n")
	rsh1 := (*reflect.SliceHeader)(unsafe.Pointer(&slice1))
	rsh2 := (*reflect.SliceHeader)(unsafe.Pointer(&slice2))
	rsh3 := (*reflect.SliceHeader)(unsafe.Pointer(ptr))
	fmt.Printf("  All data pointers equal: %v\n", rsh1.Data == rsh2.Data && rsh2.Data == rsh3.Data)

	// The critical test
	if rsh1.Data != rsh2.Data || rsh2.Data != rsh3.Data {
		fmt.Printf("  ✗ FAIL: References don't point to same backing array\n")
		return false
	}

	// Verify modification propagates
	slice1[0] = 100
	if slice2[0] != 100 || (*ptr)[0] != 100 {
		fmt.Printf("  ✗ FAIL: Modification did not propagate to all references\n")
		return false
	}

	fmt.Printf("  ✓ PASS: Memory structure preserved\n")
	return true
}

func main() {
	fmt.Println("Testing current implementation for memory structure preservation...")

	passed := 0
	total := 0

	tests := []struct {
		name string
		fn   func() bool
	}{
		{"Pointer to Slice Element", testPointerToSliceElement},
		{"Shared Slice Backing Array", testSharedSliceBackingArray},
		{"Pointer Chain", testPointerChain},
		{"Multiple References to Same Slice", testMultipleReferencesToSameSlice},
	}

	for _, test := range tests {
		total++
		if test.fn() {
			passed++
		}
	}

	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Passed: %d/%d (%.1f%%)\n", passed, total, float64(passed)/float64(total)*100)

	if passed == total {
		fmt.Println("✓ All memory structure tests passed!")
	} else {
		fmt.Println("✗ Some memory structure tests failed - implementation needs fixes")
	}
}
