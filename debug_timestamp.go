package main

import (
	"encoding/json"
	"fmt"
	"time"

	"gohypo/domain/core"
)

func main() {
	t := time.Now()
	ts := core.NewTimestamp(t)

	fmt.Printf("Original time: %v\n", t)
	fmt.Printf("Timestamp: %v\n", ts)

	// Test marshaling
	data, err := json.Marshal(ts)
	if err != nil {
		fmt.Printf("Error marshaling: %v\n", err)
		return
	}
	fmt.Printf("JSON: %s\n", string(data))

	// Test with struct
	type TestStruct struct {
		ObservedAt core.Timestamp `json:"observed_at"`
	}

	test := TestStruct{ObservedAt: ts}
	data2, err := json.Marshal(test)
	if err != nil {
		fmt.Printf("Error marshaling struct: %v\n", err)
		return
	}
	fmt.Printf("Struct JSON: %s\n", string(data2))
}
