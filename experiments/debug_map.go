package main

import (
	"fmt"

	"github.com/URALINNOVATSIYA/codec"
)

func main() {
	m := map[string]int{"one": 1}
	data := codec.Serialize(m)
	fmt.Printf("Map serialized: %v\n", data)
	fmt.Printf("Hex: %x\n", data)

	// Try to decode
	result, err := codec.Unserialize(data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success: %v\n", result)
	}
}
