package main

import (
	"fmt"
	"math"
)

func main() {
	n := int64(9007199791611905)
	u := uint64(9007199791611905)
	fmt.Println(math.Float64bits(float64(float32(n))), math.Float64bits(float64(float32(u))))
	fmt.Println(math.Float64bits(float64(float32(-n))), math.Float64bits(float64(float32(^uint64(0)))))
}
