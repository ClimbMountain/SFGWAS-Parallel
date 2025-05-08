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

	rtype := mpc_core.LElem2NBigIntZero
	rtype.SetModulusPowerOf2(63)
	fracBits := mpc.GetFracBits()

	// Number of values to test
	N := 100
	some_large_number := 1e4 // Change as needed
	x := make([]float64, N)
	expected := make([]float64, N)
	for i := range expected {
		x[i] = rand.Float64() * some_large_number
		expected[i] = math.Sin(x[i])
	}

	var xRV mpc_core.RVec

	// Share the values across parties
	if pid == 1 { // party 1: x - mask
		xRV = mpc_core.FloatToRVec(rtype, x, fracBits)
		mpc.Network.Rand.SwitchPRG(2) // Shared PRG with party 2
		mask := mpc.Network.Rand.RandVec(rtype, int(N))
		mpc.Network.Rand.RestorePRG()
		xRV.Sub(mask)
	} else if pid == 2 { // party 2: mask
		mpc.Network.Rand.SwitchPRG(1) // Shared PRG with party 1
		mask := mpc.Network.Rand.RandVec(rtype, int(N))
		mpc.Network.Rand.RestorePRG()
		xRV = mask
	} else {
		xRV = mpc_core.InitRVec(rtype.Zero(), int(N))
	}

	// ------------
	// Sine evaluation using the polynomial approximation
	// ------------
	// For a sine approximation of the form:
	//   sin(w) ≈ b * v * poly(v^2)
	// where:
	//   v = w * (1/(π/2)) = w * (2/π)
	//   poly(·) is our polynomial with coefficients p_3307 (which approximates sin(x)/x)
	//   b is a sign adjustment (b = s * (-2) + 1). For simplicity, we assume s = 0 so b = 1.
	// (In a complete implementation, you would perform full argument reduction to compute both w and s.)

	// --- Step 1. (Optional) Argument reduction ---
	// Here we assume xRV is already in a reduced range. (If not, perform the necessary reduction.)

	// --- Step 2. Compute v = w * (2/π) ---
	twoOverPi := 2.0 / math.Pi
	v := mpc_core.MulByConstant(xRV, twoOverPi)

	// --- Step 3. Compute v^2 ---
	// Assuming you have a vectorized multiplication (element-wise).
	v2 := mpc_core.VecMul(v, v) // This multiplies each element: v2[i] = v[i] * v[i]

	// --- Step 4. Evaluate the polynomial P(v^2) ---
	// p_3307: coefficients of the polynomial (as given)
	p_3307 := []float64{
		1.57079632679489000000000,
		-0.64596409750624600000000,
		0.07969262624616700000000,
		-0.00468175413531868000000,
		0.00016044118478735800000,
		-0.00000359884323520707000,
		0.00000005692172920657320,
		-0.00000000066880348849204,
		0.00000000000606691056085,
		-0.00000000000004375295071,
		0.00000000000000025002854,
	}
	// Use the modified evaluation function that includes the constant term.
	polyVal := mpc.EvaluatePolynomial(p_3307, v2)

	// --- Step 5. (Optional) Adjust sign ---
	// According to the paper:
	//     b = s * (-2) + 1,
	// where s is computed from the reduction (s=0 gives b=1). For now, we set b=1.
	b := mpc_core.InitRVec(rtype.One(), len(xRV))

	// --- Step 6. Combine to get sine: sin(w) ≈ b * v * polyVal ---
	sinApprox := mpc_core.VecMul(v, polyVal)
	sinApprox = mpc_core.VecMul(b, sinApprox)

	// Reveal the result and convert to float:
	computed := mpc.RevealSymVec(sinApprox).ToFloat(2 * fracBits)

	// Only party 1 does the testing and printing
	if pid == 1 {
		errorTol := 1e-4
		success := true
		for i := range computed {
			relativeError := math.Abs((computed[i] - expected[i]) / expected[i])
			if relativeError > errorTol {
				success = false
				fmt.Printf("Value %d: Computed = %f, Expected = %f, Relative Error = %e\n",
					i, computed[i], expected[i], relativeError)
			}
		}
		if success {
			fmt.Println("Sine computation test (using polynomial evaluation): success")
		} else {
			fmt.Println("Sine computation test (using polynomial evaluation): failed")
		}
	}
	prot.SyncAndTerminate(true)
}
