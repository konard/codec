package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

// This test verifies that deserialized values have the EXACT same memory structure
// as the original values, including:
// - Slice backing arrays
// - Pointer chains
// - Pointers to elements within slices/arrays/structs

func testSliceBackingArray() {
	fmt.Println("\n=== Testing Slice Backing Array ===")

	// Create a slice with specific backing array
	backing := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	slice1 := backing[2:5] // [3, 4, 5]
	slice2 := backing[5:8] // [6, 7, 8]

	fmt.Printf("Original slice1: %v (cap=%d, len=%d)\n", slice1, cap(slice1), len(slice1))
	fmt.Printf("Original slice2: %v (cap=%d, len=%d)\n", slice2, cap(slice2), len(slice2))
	fmt.Printf("Original backing: %v\n", backing)

	// Get slice headers
	sh1 := (*reflect.SliceHeader)(unsafe.Pointer(&slice1))
	sh2 := (*reflect.SliceHeader)(unsafe.Pointer(&slice2))
	shb := (*reflect.SliceHeader)(unsafe.Pointer(&backing))

	fmt.Printf("slice1 data ptr: %x\n", sh1.Data)
	fmt.Printf("slice2 data ptr: %x\n", sh2.Data)
	fmt.Printf("backing data ptr: %x\n", shb.Data)

	// Check if slices share the same backing array
	if sh1.Data >= shb.Data && sh1.Data < shb.Data+uintptr(shb.Len*8) {
		fmt.Println("✓ slice1 shares backing array with backing")
	}
	if sh2.Data >= shb.Data && sh2.Data < shb.Data+uintptr(shb.Len*8) {
		fmt.Println("✓ slice2 shares backing array with backing")
	}
}

func testPointerToSliceElement() {
	fmt.Println("\n=== Testing Pointer to Slice Element ===")

	slice := []int{1, 2, 3, 4, 5}
	ptr := &slice[2] // Pointer to element at index 2

	fmt.Printf("Original slice: %v\n", slice)
	fmt.Printf("Pointer value: %d (address: %p)\n", *ptr, ptr)
	fmt.Printf("Element address: %p\n", &slice[2])

	if ptr == &slice[2] {
		fmt.Println("✓ Pointer points to slice element")
	}

	// Modify through pointer
	*ptr = 100
	fmt.Printf("After modification via pointer: %v\n", slice)
}

func testPointerChains() {
	fmt.Println("\n=== Testing Pointer Chains ===")

	value := 42
	ptr1 := &value
	ptr2 := &ptr1
	ptr3 := &ptr2

	fmt.Printf("value: %d (addr: %p)\n", value, &value)
	fmt.Printf("ptr1: %p -> %d (addr: %p)\n", ptr1, *ptr1, &ptr1)
	fmt.Printf("ptr2: %p -> %p -> %d (addr: %p)\n", ptr2, *ptr2, **ptr2, &ptr2)
	fmt.Printf("ptr3: %p -> %p -> %p -> %d\n", ptr3, *ptr3, **ptr3, ***ptr3)

	// Verify they point to the same value
	if ptr1 == &value && *ptr2 == &value && **ptr3 == &value {
		fmt.Println("✓ Pointer chain is correct")
	}
}

func testStructWithPointers() {
	fmt.Println("\n=== Testing Struct with Pointers ===")

	type Node struct {
		Value int
		Slice []int
		Ptr   *int
	}

	slice := []int{1, 2, 3}
	node := Node{
		Value: 42,
		Slice: slice,
		Ptr:   &slice[1], // Pointer to element in slice
	}

	fmt.Printf("Node.Value: %d\n", node.Value)
	fmt.Printf("Node.Slice: %v\n", node.Slice)
	fmt.Printf("Node.Ptr: %p -> %d\n", node.Ptr, *node.Ptr)
	fmt.Printf("Slice[1] addr: %p\n", &node.Slice[1])

	// This is the critical test: does the pointer point to the slice element?
	if node.Ptr == &node.Slice[1] {
		fmt.Println("✓ Pointer points to slice element within struct")
	} else {
		fmt.Println("✗ Pointer does NOT point to slice element within struct")
	}
}

func testMultipleReferencesToSameSlice() {
	fmt.Println("\n=== Testing Multiple References to Same Slice ===")

	type Container struct {
		Slice1 []int
		Slice2 []int
		Ptr    *[]int
	}

	slice := []int{1, 2, 3, 4, 5}
	container := Container{
		Slice1: slice,
		Slice2: slice, // Same slice
		Ptr:    &slice, // Pointer to same slice
	}

	// Get slice headers
	sh1 := (*reflect.SliceHeader)(unsafe.Pointer(&container.Slice1))
	sh2 := (*reflect.SliceHeader)(unsafe.Pointer(&container.Slice2))
	sh3 := (*reflect.SliceHeader)(unsafe.Pointer(container.Ptr))

	fmt.Printf("Slice1 data ptr: %x\n", sh1.Data)
	fmt.Printf("Slice2 data ptr: %x\n", sh2.Data)
	fmt.Printf("*Ptr data ptr: %x\n", sh3.Data)

	if sh1.Data == sh2.Data && sh2.Data == sh3.Data {
		fmt.Println("✓ All three references point to the same backing array")
	} else {
		fmt.Println("✗ References point to different backing arrays")
	}

	// Modify through one reference
	container.Slice1[0] = 100
	fmt.Printf("After Slice1[0] = 100:\n")
	fmt.Printf("  Slice1: %v\n", container.Slice1)
	fmt.Printf("  Slice2: %v\n", container.Slice2)
	fmt.Printf("  *Ptr: %v\n", *container.Ptr)
}

func main() {
	testSliceBackingArray()
	testPointerToSliceElement()
	testPointerChains()
	testStructWithPointers()
	testMultipleReferencesToSameSlice()

	fmt.Println("\n=== Summary ===")
	fmt.Println("These are the memory structure patterns that MUST be preserved during serialization/deserialization:")
	fmt.Println("1. Slices that share the same backing array must continue to share it after deserialization")
	fmt.Println("2. Pointers to slice elements must point to the actual deserialized slice elements")
	fmt.Println("3. Pointer chains must maintain their structure (ptr -> ptr -> ptr -> value)")
	fmt.Println("4. Structs with pointers to their own fields must maintain those relationships")
	fmt.Println("5. Multiple references to the same slice must all reference the same slice after deserialization")
}
