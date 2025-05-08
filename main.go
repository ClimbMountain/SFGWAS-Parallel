package main

import (
	"fmt"
	"math"

	mpc_core "github.com/hhcho/mpc-core"
)

func main() {
	// Create new LElem2Pi elements with values and a factor k = 2
	elem1 := mpc_core.LElem2Pi{Val: 8*math.Pi - 2, K: 2} // π radians
	elem2 := mpc_core.LElem2Pi{Val: math.Pi / 2, K: 2}   // π/2 radians

	// Perform arithmetic operations
	sum := elem1.Add(elem2).(mpc_core.LElem2Pi)
	diff := elem1.Sub(elem2).(mpc_core.LElem2Pi)
	prod := elem1.Mul(elem2).(mpc_core.LElem2Pi)
	neg := elem1.Neg().(mpc_core.LElem2Pi)
	sin := elem1.Sin()

	// Print results
	fmt.Printf("Sum: %f\n", sum.Val)
	fmt.Printf("Difference: %f\n", diff.Val)
	fmt.Printf("Product: %f\n", prod.Val)
	fmt.Printf("Negation: %f\n", neg.Val)
	fmt.Printf("Sine: %f\n", sin)
}
