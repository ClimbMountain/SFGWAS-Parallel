// package main

// import (
// 	"fmt"
// 	"math"
// 	"math/rand"
// 	"os"
// 	"path/filepath"
// 	"runtime"
// 	"strconv"

// 	// Import the color package
// 	"github.com/BurntSushi/toml"
// 	"github.com/hhcho/sfgwas-private/gwas"

// 	mpc_core "github.com/hhcho/mpc-core"
// )

// // Default config path
// var CONFIG_PATH = "config/"

// // Expects a party ID provided as an environment variable;
// // e.g., run "PID=1 go run sfgwas.go"
// var PID, PID_ERR = strconv.Atoi(os.Getenv("PID"))

// func main() {
// 	RunSinGraph()
// }

// func InitProtocol(configPath string, mpcOnly bool) *gwas.ProtocolInfo {
// 	config := new(gwas.Config)

// 	// Import global parameters
// 	if _, err := toml.DecodeFile(filepath.Join(configPath, "configGlobal.toml"), config); err != nil {
// 		fmt.Println(err)
// 		return nil
// 	}

// 	// Import local parameters
// 	if _, err := toml.DecodeFile(filepath.Join(configPath, fmt.Sprintf("configLocal.Party%d.toml", PID)), config); err != nil {
// 		fmt.Println(err)
// 		return nil
// 	}

// 	// Create cache/output directories
// 	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
// 		panic(err)
// 	}
// 	if err := os.MkdirAll(config.OutDir, 0755); err != nil {
// 		panic(err)
// 	}

// 	// Set max number of threads
// 	runtime.GOMAXPROCS(config.LocalNumThreads)

// 	return gwas.InitializeGWASProtocol(config, PID, mpcOnly)
// }

// func RunSinGraph() {
// 	if PID_ERR != nil {
// 		panic(PID_ERR)
// 	}

// 	// Initialize protocol
// 	prot := InitProtocol(CONFIG_PATH, true)

// 	mpc := prot.GetMpc()[0]
// 	pid := mpc.GetPid()

// 	rtype := mpc_core.LElem256Zero
// 	fracBits := mpc.GetFracBits()

// 	N := 100
// 	x := make([]float64, N)
// 	expected := make([]float64, N)

// 	// Keep x in [0, 2π] (DO NOT shift by π)
// 	for i := range expected {
// 		x[i] = rand.Float64() * 2 * math.Pi
// 		expected[i] = math.Sin(x[i]) // Expected values of sin(x)
// 	}

// 	var xRV mpc_core.RVec

// 	if pid == 1 { // Set shares to: x - mask
// 		xRV = mpc_core.FloatToRVec(rtype, x, fracBits)

// 		mpc.Network.Rand.SwitchPRG(2) // Shared PRG between 1 and 2
// 		mask := mpc.Network.Rand.RandVec(rtype, int(N))
// 		mpc.Network.Rand.RestorePRG()

// 		xRV.Sub(mask)

// 	} else if pid == 2 { // Set shares to: mask
// 		mpc.Network.Rand.SwitchPRG(1) // Shared PRG between 1 and 2
// 		mask := mpc.Network.Rand.RandVec(rtype, int(N))
// 		mpc.Network.Rand.RestorePRG()

// 		xRV = mask

// 	} else {
// 		xRV = mpc_core.InitRVec(rtype.Zero(), int(N))
// 	}

// 	// Correct Remez polynomial coefficients (excluding c_0)
// 	coefficients := []float64{
// 		0,
// 		1.00000853,      // x^1
// 		-6.16627144e-05, // x^2
// 		-1.66474822e-01, // x^3
// 		-3.22434328e-04, // x^4
// 		8.66199606e-03,  // x^5
// 		-2.16719119e-04, // x^6
// 		-1.02757146e-04, // x^7
// 		-2.85631692e-05, // x^8
// 		8.43837175e-06,  // x^9
// 		-7.09336675e-07, // x^10
// 		2.05262615e-08,  // x^11
// 	}

// 	// Compute sine approximation using the Remez polynomial
// 	s := mpc.EvaluatePolynomial(coefficients, xRV)

// 	// Reveal results and convert back to float with increased precision
// 	computed := mpc.RevealSymVec(s).ToFloat(2 * fracBits) // Increased precision

// 	// Add back the c_0 term after revealing
// 	c0 := -2.82532278e-07
// 	for i := range computed {
// 		computed[i] += c0
// 	}

// 	if pid == 1 {
// 		totalError := 0.0
// 		totalSquaredError := 0.0
// 		totalAbsoluteError := 0.0

// 		// Compute MSE, MAE, and Average Relative Error
// 		for i := range computed {
// 			// fmt.Printf("Computed: %.12f, Expected: %.12f\n", computed[i], expected[i])
// 			relativeError := math.Abs((computed[i] - expected[i]) / expected[i])
// 			squaredError := math.Pow(computed[i]-expected[i], 2)
// 			absoluteError := math.Abs(computed[i] - expected[i])

// 			totalError += relativeError
// 			totalSquaredError += squaredError
// 			totalAbsoluteError += absoluteError
// 		}

// 		// Compute final error metrics
// 		avgError := totalError / float64(N)
// 		mse := totalSquaredError / float64(N)
// 		mae := totalAbsoluteError / float64(N)

// 		// Print results with high precision
// 		fmt.Printf("Remez Polynomial Approximation:\n")
// 		fmt.Printf("Average Relative Error: %.12f\n", avgError)
// 		fmt.Printf("Mean Squared Error (MSE): %.12f\n", mse)
// 		fmt.Printf("Mean Absolute Error (MAE): %.12f\n", mae)
// 	}

// 	prot.SyncAndTerminate(true)
// }

// package main

// import (
// 	"fmt"
// 	"math"
// 	"math/rand"
// 	"os"
// 	"path/filepath"
// 	"runtime"
// 	"strconv"

// 	// Import the color package
// 	"github.com/BurntSushi/toml"
// 	"github.com/hhcho/sfgwas-private/gwas"

// 	mpc_core "github.com/hhcho/mpc-core"
// )

// // Default config path
// var CONFIG_PATH = "config/"

// // Expects a party ID provided as an environment variable;
// // e.g., run "PID=1 go run sfgwas.go"
// var PID, PID_ERR = strconv.Atoi(os.Getenv("PID"))

// func main() {
// 	RunSinGraph()
// }

// func InitProtocol(configPath string, mpcOnly bool) *gwas.ProtocolInfo {
// 	config := new(gwas.Config)

// 	// Import global parameters
// 	if _, err := toml.DecodeFile(filepath.Join(configPath, "configGlobal.toml"), config); err != nil {
// 		fmt.Println(err)
// 		return nil
// 	}

// 	// Import local parameters
// 	if _, err := toml.DecodeFile(filepath.Join(configPath, fmt.Sprintf("configLocal.Party%d.toml", PID)), config); err != nil {
// 		fmt.Println(err)
// 		return nil
// 	}

// 	// Create cache/output directories
// 	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
// 		panic(err)
// 	}
// 	if err := os.MkdirAll(config.OutDir, 0755); err != nil {
// 		panic(err)
// 	}

// 	// Set max number of threads
// 	runtime.GOMAXPROCS(config.LocalNumThreads)

// 	return gwas.InitializeGWASProtocol(config, PID, mpcOnly)
// }

// func RunSinGraph() {
// 	if PID_ERR != nil {
// 		panic(PID_ERR)
// 	}

// 	// Initialize protocol
// 	prot := InitProtocol(CONFIG_PATH, true)

// 	mpc := prot.GetMpc()[0]
// 	pid := mpc.GetPid()

// 	rtype := mpc_core.LElem2NBigIntZero
// 	rtype.SetModulusPowerOf2(63)

// 	fracBits := mpc.GetFracBits()

// 	N := 100000
// 	// some_large_number := 1e4 // Change this value as needed
// 	x := make([]float64, N)
// 	expected := make([]float64, N)
// 	for i := range expected {
// 		x[i] = rand.Float64() * 2 * math.Pi // Random float in [0, some_large_number]
// 		expected[i] = math.Sin(x[i])
// 	}

// 	var xRV mpc_core.RVec

// 	if pid == 1 { // Set shares to: x - mask
// 		xRV = mpc_core.FloatToRVec(rtype, x, fracBits)

// 		mpc.Network.Rand.SwitchPRG(2) // Shared PRG between 1 and 2
// 		mask := mpc.Network.Rand.RandVec(rtype, int(N))
// 		mpc.Network.Rand.RestorePRG()

// 		xRV.Sub(mask)

// 	} else if pid == 2 { // Set shares to: mask
// 		mpc.Network.Rand.SwitchPRG(1) // Shared PRG between 1 and 2
// 		mask := mpc.Network.Rand.RandVec(rtype, int(N))
// 		mpc.Network.Rand.RestorePRG()

// 		xRV = mask

// 	} else {
// 		xRV = mpc_core.InitRVec(rtype.Zero(), int(N))
// 	}

// 	// s, _ := mpc.SSTrigVec(xRV)
// 	iterations := 100
// 	s, _, avgElapsed := mpc.SSTrigVecRepeated(xRV, iterations)
// 	fmt.Printf("Average SSTrigVec computation time over %d iterations: %v\n", iterations, avgElapsed)

// 	computed := mpc.RevealSymVec(s).ToFloat(2 * fracBits)

// 	if pid == 1 {
// 		totalError := 0.0
// 		totalSquaredError := 0.0
// 		totalAbsoluteError := 0.0

// 		// Compute MSE, MAE, and Average Relative Error
// 		for i := range computed {
// 			relativeError := math.Abs((computed[i] - expected[i]) / expected[i])
// 			squaredError := math.Pow(computed[i]-expected[i], 2)
// 			absoluteError := math.Abs(computed[i] - expected[i])

// 			totalError += relativeError
// 			totalSquaredError += squaredError
// 			totalAbsoluteError += absoluteError
// 		}

// 		// Compute final error metrics
// 		avgError := totalError / float64(N)
// 		mse := totalSquaredError / float64(N)
// 		mae := totalAbsoluteError / float64(N)

// 		// Print results with high precision
// 		fmt.Printf("Average Relative Error: %.12f\n", avgError)
// 		fmt.Printf("Mean Squared Error (MSE): %.12f\n", mse)
// 		fmt.Printf("Mean Absolute Error (MAE): %.12f\n", mae)
// 	}

// 	prot.SyncAndTerminate(true)
// }

package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	// Import the color package
	"github.com/BurntSushi/toml"
	"github.com/hhcho/sfgwas-private/gwas"

	mpc_core "github.com/hhcho/mpc-core"
)

// Default config path
var CONFIG_PATH = "config/"

// Expects a party ID provided as an environment variable;
// e.g., run "PID=1 go run sfgwas.go"
var PID, PID_ERR = strconv.Atoi(os.Getenv("PID"))

func main() {
	RunSinGraph()
}

func InitProtocol(configPath string, mpcOnly bool) *gwas.ProtocolInfo {
	config := new(gwas.Config)

	// Import global parameters
	if _, err := toml.DecodeFile(filepath.Join(configPath, "configGlobal.toml"), config); err != nil {
		fmt.Println(err)
		return nil
	}

	// Import local parameters
	if _, err := toml.DecodeFile(filepath.Join(configPath, fmt.Sprintf("configLocal.Party%d.toml", PID)), config); err != nil {
		fmt.Println(err)
		return nil
	}

	// Create cache/output directories
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(config.OutDir, 0755); err != nil {
		panic(err)
	}

	// Set max number of threads
	runtime.GOMAXPROCS(config.LocalNumThreads)

	return gwas.InitializeGWASProtocol(config, PID, mpcOnly)
}

func RunSinGraph() {
	if PID_ERR != nil {
		panic(PID_ERR)
	}

	// Initialize protocol
	prot := InitProtocol(CONFIG_PATH, true)

	mpc := prot.GetMpc()[0]
	pid := mpc.GetPid()

	rtype := mpc_core.LElem256Zero
	fracBits := mpc.GetFracBits()

	N := 2000
	x := make([]float64, N)
	expected := make([]float64, N)

	for i := range expected {
		x[i] = rand.Float64() * 2 * math.Pi // Keep x in [0, 2π]
		// fmt.Printf("x : %f\n", x[i])
		shiftedX := x[i] - math.Pi   // Shift x by π
		expected[i] = math.Sin(x[i]) // Expected sin(x) values
		x[i] = shiftedX              // Store shifted values in x[]
	}

	var xRV mpc_core.RVec

	if pid == 1 { // Set shares to: x - mask
		xRV = mpc_core.FloatToRVec(rtype, x, fracBits)

		mpc.Network.Rand.SwitchPRG(2) // Shared PRG between 1 and 2
		mask := mpc.Network.Rand.RandVec(rtype, int(N))
		mpc.Network.Rand.RestorePRG()

		xRV.Sub(mask)

	} else if pid == 2 { // Set shares to: mask
		mpc.Network.Rand.SwitchPRG(1) // Shared PRG between 1 and 2
		mask := mpc.Network.Rand.RandVec(rtype, int(N))
		mpc.Network.Rand.RestorePRG()

		xRV = mask

	} else {
		xRV = mpc_core.InitRVec(rtype.Zero(), int(N))
	}

	// Use Taylor series coefficients for sin(x) up to x^21 / 21!
	coefficients := []float64{
		0.0,                 // x^0 term (unused)
		-1.0,                // x^1 / 1!
		0.0,                 // x^2 term (not used)
		1.0 / 6.0,           // x^3 / 3!
		0.0,                 // x^4 term (not used)
		-1.0 / 120.0,        // x^5 / 5!
		0.0,                 // x^6 term (not used)
		1.0 / 5040.0,        // x^7 / 7!
		0.0,                 // x^8 term (not used)
		-1.0 / 362880.0,     // x^9 / 9!
		0.0,                 // x^10 term (not used)
		1.0 / 39916800.0,    // x^11 / 11!
		0.0,                 // x^12 term (not used)
		-1.0 / 6227020800.0, // x^13 / 13!
		// 0.0,                           // x^14 term (not used)
		// 1.0 / 1307674368000.0,         // x^15 / 15!
		// 0.0,                           // x^16 term (not used)
		// -1.0 / 355687428096000.0,      // x^17 / 17!
		// 0.0,                           // x^18 term (not used)
		// 1.0 / 121645100408832000.0,    // x^19 / 19!
		// 0.0,                           // x^20 term (not used)
		// -1.0 / 51090942171709440000.0, // x^21 / 21!
	}

	// Compute sine approximation using polynomial evaluation
	s := mpc.EvaluatePolynomial(coefficients, xRV)

	computed := mpc.RevealSymVec(s).ToFloat(2 * fracBits)

	if pid == 1 {
		totalError := 0.0
		totalSquaredError := 0.0
		totalAbsoluteError := 0.0

		// Compute MSE, MAE, and Average Relative Error
		for i := range computed {
			// fmt.Printf("Computed: %f\n", computed[i])
			// fmt.Printf("Expected: %f\n", expected[i])
			relativeError := math.Abs((computed[i] - expected[i]) / expected[i])
			squaredError := math.Pow(computed[i]-expected[i], 2)
			absoluteError := math.Abs(computed[i] - expected[i])

			totalError += relativeError
			totalSquaredError += squaredError
			totalAbsoluteError += absoluteError
		}

		// Compute final error metrics
		avgError := totalError / float64(N)
		mse := totalSquaredError / float64(N)
		mae := totalAbsoluteError / float64(N)

		// Print results with high precision
		fmt.Printf("Average Relative Error: %.12f\n", avgError)
		fmt.Printf("Mean Squared Error (MSE): %.12f\n", mse)
		fmt.Printf("Mean Absolute Error (MAE): %.12f\n", mae)
	}

	prot.SyncAndTerminate(true)
}
