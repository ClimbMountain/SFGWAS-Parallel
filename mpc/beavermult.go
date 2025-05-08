package mpc

import (
	"math"
	"sync"

	// for Job, CellJob
	mpc_core "github.com/hhcho/mpc-core"
)

func (mpcObj *MPC) BeaverPartition(a mpc_core.RElem) (mpc_core.RElem, mpc_core.RElem) {
	ar, am := mpcObj.BeaverPartitionMat(mpc_core.RMat{mpc_core.RVec{a}})
	return ar[0][0], am[0][0]
}

func (mpcObj *MPC) BeaverPartitionVec(a mpc_core.RVec) (mpc_core.RVec, mpc_core.RVec) {
	ar, am := mpcObj.BeaverPartitionMat(mpc_core.RMat{a})
	return ar[0], am[0]
}

/*
Each party has a matrix of shares
*/

// Input: matrix of shares, look above
// Output: Returns ar, am (x - a, a), respsectively

// starts with x

// [x] + a_1 + a_2 = am (party 0 returns)
// [x] - a_1, a_2 (party 1 returns)
// [x] - a_2, a_2 (party 2 returns)
func (mpcObj *MPC) BeaverPartitionMat(a mpc_core.RMat) (mpc_core.RMat, mpc_core.RMat) {
	pid := mpcObj.Network.pid
	nrows, ncols := a.Dims()
	rtype := a.Type()

	// Party 0 (hub) generates and sums all masks into 'am'
	if pid == 0 {
		am := mpc_core.InitRMat(rtype.Zero(), nrows, ncols)
		for p := 1; p < mpcObj.Network.NumParties; p++ {
			mpcObj.Network.Rand.SwitchPRG(p)
			mask := mpcObj.Network.Rand.RandMat(rtype, nrows, ncols)
			mpcObj.Network.Rand.RestorePRG()
			am.Add(mask)
		}
		ar := mpc_core.InitRMat(rtype.Zero(), nrows, ncols)
		return ar, am
	}

	// Other parties sample the same mask
	mpcObj.Network.Rand.SwitchPRG(0)
	mask := mpcObj.Network.Rand.RandMat(rtype, nrows, ncols)
	mpcObj.Network.Rand.RestorePRG()

	// Compute ar = a - mask in parallel, one row per goroutine
	ar := a.Copy()
	var wg sync.WaitGroup
	wg.Add(nrows)
	for i := 0; i < nrows; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < ncols; j++ {
				ar[i][j] = ar[i][j].Sub(mask[i][j])
			}
		}(i)
	}
	wg.Wait()

	// Reveal the ar shares in one batch
	ar = mpcObj.RevealSymMat(ar)

	return ar, mask
}

func (mpcObj *MPC) BeaverReconstruct(a mpc_core.RElem) mpc_core.RElem {
	return mpcObj.BeaverReconstructMat(mpc_core.RMat{mpc_core.RVec{a}})[0][0]
}

func (mpcObj *MPC) BeaverReconstructVec(a mpc_core.RVec) mpc_core.RVec {
	return mpcObj.BeaverReconstructMat(mpc_core.RMat{a})[0]
}

// For Beaver Triples, before we enter this function (Happens in BeaverMultMat):
// For party 0: a contains the value of am * bm (last element of the Beaver triple)
// For party 1: a contains the value of ar * [bm] + [am] * br + ar * br
// For party 2: a contains the value ar * [bm] + [am] * br
// Function Purpose: For party 0: secret sharing of am * bm (the triple is (am, bm, am * bm))
// party 0 calculates ar * [bm] + [am] * br + ar * br + [c] (with his shares from 0)
// party 1 calculates ar * [bm] + [am] * br + [c] (with his shares from 0)
func (mpcObj *MPC) BeaverReconstructMat(a mpc_core.RMat) mpc_core.RMat {
	pid := mpcObj.Network.pid

	rtype := a.Type()
	nr, nc := a.Dims()

	last := mpcObj.Network.NumParties - 1

	// party 0 holds the value of am * bm, we secret share it here
	if pid == 0 {

		// take our copy and subtract off random value for party 1 (store this share in PRG 1)
		// send remaining value to party 2 and return that value
		mask := a.Copy()
		for to := 1; to < mpcObj.Network.NumParties-1; to++ {
			mpcObj.Network.Rand.SwitchPRG(to)
			share := mpcObj.Network.Rand.RandMat(rtype, nr, nc) // generates their shares of c
			mpcObj.Network.Rand.RestorePRG()
			mask.Sub(share)
		}
		mpcObj.Network.SendRData(mask, last)
		return mask
	}

	var mask mpc_core.RMat

	// party 2 recieves the the share from party 0
	if pid == last {
		mask = mpcObj.Network.ReceiveRMat(rtype, nr, nc, 0)
	} else {
		mpcObj.Network.Rand.SwitchPRG(0)
		mask = mpcObj.Network.Rand.RandMat(rtype, nr, nc) // retrieve their shares of c
		mpcObj.Network.Rand.RestorePRG()
	}

	ar := a.Copy()
	ar.Add(mask)

	return ar
}

func (mpcObj *MPC) BeaverMult(ar, am, br, bm mpc_core.RElem) mpc_core.RElem {
	pid := mpcObj.Network.pid
	if pid == 0 {
		return am.Mul(bm)
	}

	out := ar.Mul(bm)
	out = out.Add(br.Mul(am))
	if pid == 1 {
		out = out.Add(ar.Mul(br))
	}
	return out
}

func (mpcObj *MPC) BeaverMultElemVec(ar, am, br, bm mpc_core.RVec) mpc_core.RVec {
	return mpcObj.BeaverMultElemMat(mpc_core.RMat{ar}, mpc_core.RMat{am}, mpc_core.RMat{br}, mpc_core.RMat{bm})[0]
}

func (mpcObj *MPC) BeaverMultElemMat(ar, am, br, bm mpc_core.RMat) mpc_core.RMat {
	pid := mpcObj.Network.pid
	nr, nc := am.Dims()

	// Party 0 just does a plaintext multiply
	if pid == 0 {
		return mpc_core.RMultMat(am, bm)
	}

	// Initialize the output matrix
	out := mpc_core.InitRMat(am.Type().Zero(), nr, nc)

	// Set up the global job queue
	totalTasks := nr * nc
	numWorkers := mpcObj.Network.NumParties // or LocalNumThreads, as desired

	jobQueue := make(chan Job, totalTasks)
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go func() {
			defer wg.Done()
			for job := range jobQueue {
				job.Execute()
			}
		}()
	}

	// Enqueue one CellJob per matrix element
	for i := 0; i < nr; i++ {
		for j := 0; j < nc; j++ {
			jobQueue <- CellJob{
				Ar:  ar,
				Am:  am,
				Br:  br,
				Bm:  bm,
				Out: out,
				I:   i,
				J:   j,
				Pid: pid,
			}
		}
	}
	close(jobQueue)

	// Wait for everything to finish
	wg.Wait()
	return out
}

// Before we enter this function (happens in BeaverPartitionMat):
// Party 0 samples (am, bm) (first two values in Beaver Triple)
// ar = a - am
// am
// br = b - bm
// bm
// Party 0 releases ar, br just like in Beaver Triples

// This function returns a matrix where
// For party 0 : returns am * bm (third value in Beaver Triple)
// For party 1: returns the value of ar * [bm] + [am] * br + ar * br
// For party 2: returns the value ar * [bm] + [am] * br
func (mpcObj *MPC) BeaverMultMat(ar, am, br, bm mpc_core.RMat) mpc_core.RMat {
	pid := mpcObj.Network.pid
	if pid == 0 {
		//return am * bm if party 0
		return mpc_core.RMultMat(am, bm)
	}

	//return (x-a)[b] + (y-b)[a] + [c] + (x-a)(y-b) if not party 0
	out := mpc_core.RMultMat(ar, bm)
	out.Add(mpc_core.RMultMat(am, br))
	if pid == 1 {
		out.Add(mpc_core.RMultMat(ar, br))
	}
	return out
}

func (mpcObj *MPC) BeaverSinCos(ar, am mpc_core.RElem) (mpc_core.RElem, mpc_core.RElem) {

	pid := mpcObj.Network.pid
	rtype := mpcObj.GetRType().Zero()
	fracBits := mpcObj.GetFracBits()

	last := mpcObj.Network.NumParties - 1

	if pid == 0 {

		sin_am, cos_am := am.Copy(), am.Copy()
		// Turn the am into a float
		sin_am_float := sin_am.Float64(fracBits)
		cos_am_float := cos_am.Float64(fracBits)

		// Take sin of float and turn the sin and cos back into ring elements
		sin_am = rtype.FromFloat64(math.Sin(sin_am_float), fracBits)
		cos_am = rtype.FromFloat64(math.Cos(cos_am_float), fracBits)

		// fmt.Printf("value of sin(am): %v\n", sin_am.Float64(fracBits))
		// fmt.Printf("value of cos(am) %v\n", cos_am.Float64(fracBits))

		// take our copy and subtract off random value for party 1 (store this share in PRG 1)
		// send remaining value to party 2 and return that value
		mask_sin := sin_am.Copy()
		mask_cos := cos_am.Copy()
		for to := 1; to < mpcObj.Network.NumParties-1; to++ {
			mpcObj.Network.Rand.SwitchPRG(to)

			share_sin := mpcObj.Network.Rand.RandElem(rtype) // generates their shares of sine of c
			share_cos := mpcObj.Network.Rand.RandElem(rtype) // generates their shares of cos of c
			mpcObj.Network.Rand.RestorePRG()

			mask_sin = mask_sin.Sub(share_sin)
			mask_cos = mask_cos.Sub(share_cos)
		}

		mpcObj.Network.SendRData(mask_sin, last)
		mpcObj.Network.SendRData(mask_cos, last)

		return mask_sin, mask_cos
	}

	sin_ar, cos_ar := ar.Copy(), ar.Copy()
	// Turn the ar into a float
	sin_ar_float := sin_ar.Float64(fracBits)
	cos_ar_float := cos_ar.Float64(fracBits)

	// Take sin of float and turn the sin and cos back into ring elements
	sin_ar = rtype.FromFloat64(math.Sin(sin_ar_float), fracBits)
	cos_ar = rtype.FromFloat64(math.Cos(cos_ar_float), fracBits)

	// fmt.Printf("sin_ar is %v\n", sin_ar.Float64(fracBits))
	// fmt.Printf("cos_ar is %v\n", cos_ar.Float64(fracBits))

	var mask_sin mpc_core.RElem
	var mask_cos mpc_core.RElem

	// party 2 recieves the share from party 0
	if pid == last {
		// shares of sin(am) and cos(am) now
		mask_sin = mpcObj.Network.ReceiveRElem(rtype, 0)
		mask_cos = mpcObj.Network.ReceiveRElem(rtype, 0)
	} else {
		mpcObj.Network.Rand.SwitchPRG(0)
		mask_sin = mpcObj.Network.Rand.RandElem(rtype) // retrieve their shares of sin of c
		mask_cos = mpcObj.Network.Rand.RandElem(rtype) // retrieve their shares of cos of c
		mpcObj.Network.Rand.RestorePRG()
	}

	// // [sin(a)]cos(x-a) + [cos(a)]sin(x-a)
	// sin := mask_sin.Mul(cos_ar).Add(mask_cos.Mul(sin_ar))

	// // [cos(a)]cos(x-a) - [sin(a)]sin(x-a)
	// cos := mask_cos.Mul(cos_ar).Sub(mask_sin.Mul(sin_ar))

	// [sin(a)]cos(x-a) + [cos(a)]sin(x-a)
	sin_first := mask_sin.Mul(cos_ar)
	sin_second := mask_cos.Mul(sin_ar)

	// [cos(a)]cos(x-a) - [sin(a)]sin(x-a)
	cos_first := mask_cos.Mul(cos_ar)
	cos_second := mask_sin.Mul(sin_ar)

	sin_first = sin_first.Add(sin_second)
	cos_first = cos_first.Sub(cos_second)

	return sin_first, cos_first
}

// func (mpcObj *MPC) BeaverSinCosVec(ar, am mpc_core.RVec) (mpc_core.RVec, mpc_core.RVec) {

// 	pid := mpcObj.Network.pid
// 	rtype := mpcObj.GetRType().Zero()
// 	fracBits := mpcObj.GetFracBits()

// 	last := mpcObj.Network.NumParties - 1

// 	if pid == 0 {

// 		sin_am, cos_am := am.Copy(), am.Copy()
// 		// Turn the am into a float
// 		sin_am_float := sin_am.Float64(fracBits)
// 		cos_am_float := cos_am.Float64(fracBits)

// 		// Take sin of float and turn the sin and cos back into ring elements
// 		sin_am = rtype.FromFloat64(math.Sin(sin_am_float), fracBits)
// 		cos_am = rtype.FromFloat64(math.Cos(cos_am_float), fracBits)

// 		// fmt.Printf("value of sin(am): %v\n", sin_am.Float64(fracBits))
// 		// fmt.Printf("value of cos(am) %v\n", cos_am.Float64(fracBits))

// 		// take our copy and subtract off random value for party 1 (store this share in PRG 1)
// 		// send remaining value to party 2 and return that value
// 		mask_sin := sin_am.Copy()
// 		mask_cos := cos_am.Copy()
// 		for to := 1; to < mpcObj.Network.NumParties-1; to++ {
// 			mpcObj.Network.Rand.SwitchPRG(to)

// 			share_sin := mpcObj.Network.Rand.RandElem(rtype) // generates their shares of sine of c
// 			share_cos := mpcObj.Network.Rand.RandElem(rtype) // generates their shares of cos of c
// 			mpcObj.Network.Rand.RestorePRG()

// 			mask_sin = mask_sin.Sub(share_sin)
// 			mask_cos = mask_cos.Sub(share_cos)
// 		}

// 		mpcObj.Network.SendRData(mask_sin, last)
// 		mpcObj.Network.SendRData(mask_cos, last)

// 		return mask_sin, mask_cos
// 	}

// 	sin_ar, cos_ar := ar.Copy(), ar.Copy()
// 	// Turn the ar into a float
// 	sin_ar_float := sin_ar.Float64(fracBits)
// 	cos_ar_float := cos_ar.Float64(fracBits)

// 	// Take sin of float and turn the sin and cos back into ring elements
// 	sin_ar = rtype.FromFloat64(math.Sin(sin_ar_float), fracBits)
// 	cos_ar = rtype.FromFloat64(math.Cos(cos_ar_float), fracBits)

// 	// fmt.Printf("sin_ar is %v\n", sin_ar.Float64(fracBits))
// 	// fmt.Printf("cos_ar is %v\n", cos_ar.Float64(fracBits))

// 	var mask_sin mpc_core.RElem
// 	var mask_cos mpc_core.RElem

// 	// party 2 recieves the share from party 0
// 	if pid == last {
// 		// shares of sin(am) and cos(am) now
// 		mask_sin = mpcObj.Network.ReceiveRElem(rtype, 0)
// 		mask_cos = mpcObj.Network.ReceiveRElem(rtype, 0)
// 	} else {
// 		mpcObj.Network.Rand.SwitchPRG(0)
// 		mask_sin = mpcObj.Network.Rand.RandElem(rtype) // retrieve their shares of sin of c
// 		mask_cos = mpcObj.Network.Rand.RandElem(rtype) // retrieve their shares of cos of c
// 		mpcObj.Network.Rand.RestorePRG()
// 	}

// 	// // [sin(a)]cos(x-a) + [cos(a)]sin(x-a)
// 	// sin := mask_sin.Mul(cos_ar).Add(mask_cos.Mul(sin_ar))

// 	// // [cos(a)]cos(x-a) - [sin(a)]sin(x-a)
// 	// cos := mask_cos.Mul(cos_ar).Sub(mask_sin.Mul(sin_ar))

// 	// [sin(a)]cos(x-a) + [cos(a)]sin(x-a)
// 	sin_first := mask_sin.Mul(cos_ar)
// 	sin_second := mask_cos.Mul(sin_ar)

// 	// [cos(a)]cos(x-a) - [sin(a)]sin(x-a)
// 	cos_first := mask_cos.Mul(cos_ar)
// 	cos_second := mask_sin.Mul(sin_ar)

// 	sin_first = sin_first.Add(sin_second)
// 	cos_first = cos_first.Sub(cos_second)

// 	return sin_first, cos_first
// }

func (mpcObj *MPC) BeaverSigmoid(ar, am mpc_core.RElem) (mpc_core.RElem, mpc_core.RElem) {
	pid := mpcObj.Network.pid
	rtype := mpcObj.GetRType().Zero()
	fracBits := mpcObj.GetFracBits()

	last := mpcObj.Network.NumParties - 1

	if pid == 0 {
		top_am := am.Copy()
		// Turn the am into a float
		top_am_float := top_am.Float64(fracBits)
		top_am_sigmoid := 1 / (1 + math.Exp(top_am_float))
		// Take sin of float and turn the sin and cos back into ring elements
		top_am = rtype.FromFloat64(top_am_sigmoid-1, fracBits)
		bot_am := rtype.FromFloat64(1-top_am_sigmoid, fracBits)
		// fmt.Printf("value of sin(am): %v\n", sin_am.Float64(fracBits))
		// fmt.Printf("value of cos(am) %v\n", cos_am.Float64(fracBits))

		// take our copy and subtract off random value for party 1 (store this share in PRG 1)
		// send remaining value to party 2 and return that value
		top_mask := top_am.Copy()
		bot_mask := bot_am.Copy()
		for to := 1; to < mpcObj.Network.NumParties-1; to++ {
			mpcObj.Network.Rand.SwitchPRG(to)

			top_share := mpcObj.Network.Rand.RandElem(rtype) // generates their shares of sine of c
			bot_share := mpcObj.Network.Rand.RandElem(rtype) // generates their shares of sine of c
			mpcObj.Network.Rand.RestorePRG()

			top_mask = top_mask.Sub(top_share)
			bot_mask = bot_mask.Sub(bot_share)
		}

		mpcObj.Network.SendRData(top_mask, last)
		mpcObj.Network.SendRData(bot_mask, last)

		return top_mask, bot_mask
	}

	top_ar := ar.Copy()
	// Turn the ar into a float
	top_ar_float := top_ar.Float64(fracBits)

	// Take sin of float and turn the sin and cos back into ring elements
	top_ar = rtype.FromFloat64(1/(1+math.Exp(-top_ar_float)), fracBits)

	// fmt.Printf("sin_ar is %v\n", sin_ar.Float64(fracBits))
	// fmt.Printf("cos_ar is %v\n", cos_ar.Float64(fracBits))

	var top_mask mpc_core.RElem
	var bot_mask mpc_core.RElem
	// party 2 recieves the share from party 0
	if pid == last {
		// shares of sin(am) and cos(am) now
		top_mask = mpcObj.Network.ReceiveRElem(rtype, 0)
		bot_mask = mpcObj.Network.ReceiveRElem(rtype, 0)
	} else {
		mpcObj.Network.Rand.SwitchPRG(0)
		top_mask = mpcObj.Network.Rand.RandElem(rtype) // retrieve their shares of sin of c
		bot_mask = mpcObj.Network.Rand.RandElem(rtype)
		mpcObj.Network.Rand.RestorePRG()
	}

	// f(x-a) and [f(-a) - 1]
	// bot_mask = f(a)

	top_first := top_mask.Mul(top_ar)

	bot_first := bot_mask.Mul(top_ar)

	bot_first = bot_first.Sub(bot_mask)

	if pid == 1 {
		bot_first.Sub(top_ar)
	}

	return top_first, bot_first
}
