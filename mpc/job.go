package mpc

import mpc_core "github.com/hhcho/mpc-core"

type Job interface{ Execute() }
type CellJob struct {
	Ar, Am, Br, Bm mpc_core.RMat
	Out            mpc_core.RMat
	I, J           int
	Pid            int
}

func (cj CellJob) Execute() {
	v := cj.Ar[cj.I][cj.J].Mul(cj.Bm[cj.I][cj.J])
	v = v.Add(cj.Br[cj.I][cj.J].Mul(cj.Am[cj.I][cj.J]))
	if cj.Pid == 1 {
		v = v.Add(cj.Ar[cj.I][cj.J].Mul(cj.Br[cj.I][cj.J]))
	}
	cj.Out[cj.I][cj.J] = v
}
