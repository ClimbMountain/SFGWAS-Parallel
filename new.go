package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	mpc_core "github.com/hhcho/mpc-core"
	"github.com/hhcho/sfgwas-private/gwas"
)

const CONFIG_PATH = "config/"

// parsePIDs converts a comma-separated string into a slice of ints
func parsePIDs(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := make([]int, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("invalid pid %q: %w", p, err)
		}
		out[i] = v
	}
	return out, nil
}

// InitProtocol initializes the GWAS protocol for a given party ID
func InitProtocol(configPath string, pid int, mpcOnly bool) *gwas.ProtocolInfo {
	config := new(gwas.Config)

	// Global parameters
	if _, err := toml.DecodeFile(filepath.Join(configPath, "configGlobal.toml"), config); err != nil {
		fmt.Println(err)
		return nil
	}

	// Local parameters for this party
	if _, err := toml.DecodeFile(
		filepath.Join(configPath, fmt.Sprintf("configLocal.Party%d.toml", pid)),
		config,
	); err != nil {
		fmt.Println(err)
		return nil
	}

	// Ensure directories exist
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(config.OutDir, 0755); err != nil {
		panic(err)
	}

	// Configure parallelism
	runtime.GOMAXPROCS(config.LocalNumThreads)

	return gwas.InitializeGWASProtocol(config, pid, mpcOnly)
}

// RunSinGraph executes the sine-approximation workload for one party
func RunSinGraph(pid int) {
	prot := InitProtocol(CONFIG_PATH, pid, true)
	mpc := prot.GetMpc()[0]

	rtype := mpc_core.LElem256Zero
	fracBits := mpc.GetFracBits()

	N := 2000
	x := make([]float64, N)
	expected := make([]float64, N)

	for i := range expected {
		x[i] = rand.Float64() * 2 * math.Pi
		shifted := x[i] - math.Pi
		expected[i] = math.Sin(x[i])
		x[i] = shifted
	}

	var xRV mpc_core.RVec

	if pid == 1 {
		xRV = mpc_core.FloatToRVec(rtype, x, fracBits)
		mpc.Network.Rand.SwitchPRG(2)
		mask := mpc.Network.Rand.RandVec(rtype, N)
		mpc.Network.Rand.RestorePRG()
		xRV.Sub(mask)

	} else if pid == 2 {
		mpc.Network.Rand.SwitchPRG(1)
		mask := mpc.Network.Rand.RandVec(rtype, N)
		mpc.Network.Rand.RestorePRG()
		xRV = mask

	} else {
		xRV = mpc_core.InitRVec(rtype.Zero(), N)
	}

	// Taylor series coefficients for sin(x)
	coeffs := []float64{
		0.0,                 // x^0
		-1.0,                // x^1/1!
		0.0,                 // x^2 unused
		1.0 / 6.0,           // x^3/3!
		0.0,                 // x^4 unused
		-1.0 / 120.0,        // x^5/5!
		0.0,                 // x^6 unused
		1.0 / 5040.0,        // x^7/7!
		0.0,                 // x^8 unused
		-1.0 / 362880.0,     // x^9/9!
		0.0,                 // x^10 unused
		1.0 / 39916800.0,    // x^11/11!
		0.0,                 // x^12 unused
		-1.0 / 6227020800.0, // x^13/13!
	}

	shares := mpc.EvaluatePolynomial(coeffs, xRV)
	computed := mpc.RevealSymVec(shares).ToFloat(2 * fracBits)

	if pid == 1 {
		totalError, totalSq, totalAbs := 0.0, 0.0, 0.0
		for i := range computed {
			relErr := math.Abs((computed[i] - expected[i]) / expected[i])
			sqErr := math.Pow(computed[i]-expected[i], 2)
			absErr := math.Abs(computed[i] - expected[i])
			totalError += relErr
			totalSq += sqErr
			totalAbs += absErr
		}

		avgErr := totalError / float64(N)
		mse := totalSq / float64(N)
		mae := totalAbs / float64(N)

		fmt.Printf("Average Relative Error: %.12f\n", avgErr)
		fmt.Printf("MSE: %.12f\n", mse)
		fmt.Printf("MAE: %.12f\n", mae)
	}

	prot.SyncAndTerminate(true)
}

func main() {
	pidsFlag := flag.String("pids", "0,1", "comma-separated party IDs to simulate in-process")
	flag.Parse()

	pids, err := parsePIDs(*pidsFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error parsing -pids:", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	for _, pid := range pids {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			RunSinGraph(pid)
		}(pid)
	}
	wg.Wait()
}
