package main

import (
	"fmt"

	"github.com/URALINNOVATSIYA/codec"
)

func main() {
	s := []int{1, 2, 3}
	data := codec.Serialize(s)
	fmt.Printf("Slice serialized: %v\n", data)
	fmt.Printf("Hex: %x\n", data)
	fmt.Printf("Length: %d\n", len(data))

	result, err := codec.Unserialize(data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success: %v\n", result)
	}
}
