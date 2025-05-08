package mpc

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/aead/chacha20/chacha"
	"github.com/hhcho/frand"
	mpc_core "github.com/hhcho/mpc-core"
	"github.com/hhcho/sfgwas-private/crypto"
	"github.com/ldsec/lattigo/v2/ckks"
	"github.com/ldsec/lattigo/v2/dckks"
	"github.com/ldsec/lattigo/v2/ring"
)

const IntBatchSize = 512

// Server holds IP/port info for InitCommunication
type Server struct {
	IpAddr string
	Ports  map[string]string
}

func pidString(pid int) string {
	return fmt.Sprintf("party%d", pid)
}

// Network is the single-threaded communication abstraction
type Network struct {
	pid, hubPid, NumParties int

	crpGen       *ring.UniformSampler
	dckksContext *dckks.Context
	Rand         *Random

	conns     map[int]net.Conn
	listeners map[int]net.Listener

	SentBytes, ReceivedBytes map[int]uint64
	commSent, commReceived   map[int]int
	loggingActive            bool

	intBuf map[int][]uint64
	intMu  map[int]*sync.Mutex

	// in the type Network { … } block, alongside intBuf/intMu:
	ctBuf    map[int][]*ckks.Ciphertext
	ctMu     map[int]*sync.Mutex
	ctThresh int

	sendChan     map[int]chan interface{} // one channel per peer
	recvChan     map[int]chan interface{} // one channel per peer
	cryptoParams *crypto.CryptoParams
}

var pipeRegistry = struct {
	sync.Mutex
	m map[string]net.Conn
}{
	m: make(map[string]net.Conn),
}

// SetMHEParams installs the CKKS/dCKKS context and CRP generator
// on this per-thread network instance.
func (n *Network) SetMHEParams(params *ckks.Parameters) {
	// Create the dCKKS context
	ctx := dckks.NewContext(params)
	n.dckksContext = ctx

	// Seed a uniform sampler for CRP
	seed := make([]byte, chacha.KeySize)
	n.Rand.SwitchPRG(-1)
	n.Rand.RandRead(seed)
	n.Rand.RestorePRG()

	prng := frand.NewCustom(seed, bufferSize, 20)
	n.crpGen = ring.NewUniformSamplerWithBasePrng(prng, ctx.RingQP)
}

// ResetNetworkLog zeros out all per-thread byte/packet counters.
func (nets ParallelNetworks) ResetNetworkLog() {
	for _, n := range nets {
		for pid := range n.SentBytes {
			n.SentBytes[pid] = 0
			n.ReceivedBytes[pid] = 0
			n.commSent[pid] = 0
			n.commReceived[pid] = 0
		}
	}
}

// PrintNetworkLog aggregates across threads and prints a summary.
func (nets ParallelNetworks) PrintNetworkLog() {
	aggSent := make(map[int]uint64)
	aggRecv := make(map[int]uint64)
	for _, n := range nets {
		for pid, b := range n.SentBytes {
			aggSent[pid] += b
		}
		for pid, b := range n.ReceivedBytes {
			aggRecv[pid] += b
		}
	}
	fmt.Printf("=== Network log for party %d ===\n", nets[0].pid)
	for pid, b := range aggSent {
		fmt.Printf("  Sent  %8d bytes to party %d\n", b, pid)
	}
	for pid, b := range aggRecv {
		fmt.Printf("  Recv  %8d bytes from party %d\n", b, pid)
	}
}

func (n *Network) EnableLogging()  { n.loggingActive = true }
func (n *Network) DisableLogging() { n.loggingActive = false }

func (n *Network) UpdateSenderLog(toPid, nbytes int) {
	if n.loggingActive {
		n.SentBytes[toPid] += uint64(nbytes)
		n.commSent[toPid]++
	}
}
func (n *Network) UpdateReceiverLog(fromPid, nbytes int) {
	if n.loggingActive {
		n.ReceivedBytes[fromPid] += uint64(nbytes)
		n.commReceived[fromPid]++
	}
}

// InitCommunication spins up one Network per thread
func InitCommunication(bindingIP string, servers map[string]Server, pid, np, threads int, sharedKeysPath string) []*Network {
	nets := make([]*Network, threads)
	var wg sync.WaitGroup
	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func(thread int) {
			defer wg.Done()
			nets[thread] = initNetworkForThread(bindingIP, servers, pid, np, thread)
			fmt.Printf("Thread %d network init\n", thread)
		}(t)
	}
	wg.Wait()
	for _, nn := range nets {
		nn.Rand = InitializePRG(pid, np, sharedKeysPath)
	}
	return nets
}

func initNetworkForThread(bindingIP string, servers map[string]Server, pid, np, thread int) *Network {
	// We no longer need bindingIP or servers for in‑process pipes
	conns := make(map[int]net.Conn)
	listeners := make(map[int]net.Listener) // still required by the struct but unused

	for other := 0; other < np; other++ {
		if other == pid {
			continue
		}

		// Build a unique key for this pair (pid<->other) on this thread
		key := fmt.Sprintf("%d-%d-%d", pid, other, thread)

		// Look up or create the pipe endpoints
		pipeRegistry.Lock()
		conn, exists := pipeRegistry.m[key]
		if !exists {
			// First time seeing this pair: create a two‑ended pipe
			c1, c2 := net.Pipe()
			pipeRegistry.m[key] = c1
			pipeRegistry.m[fmt.Sprintf("%d-%d-%d", other, pid, thread)] = c2
			conn = c1
		}
		pipeRegistry.Unlock()

		// Assign this party’s connection to "other"
		conns[other] = conn
	}

	// Construct the Network object exactly as before:
	netObj := &Network{
		pid:           pid,
		hubPid:        1,
		NumParties:    np,
		conns:         conns,
		listeners:     listeners,
		Rand:          nil,
		SentBytes:     make(map[int]uint64),
		ReceivedBytes: make(map[int]uint64),
		commSent:      make(map[int]int),
		commReceived:  make(map[int]int),
		loggingActive: true,
		intBuf:        make(map[int][]uint64, np),
		intMu:         make(map[int]*sync.Mutex, np),
	}

	netObj.ctThresh = 128
	netObj.ctBuf = make(map[int][]*ckks.Ciphertext, np)
	netObj.ctMu = make(map[int]*sync.Mutex, np)

	netObj.sendChan = make(map[int]chan interface{}, np)
	netObj.recvChan = make(map[int]chan interface{}, np)
	for i := 0; i < np; i++ {
		if i == pid {
			continue
		}
		netObj.sendChan[i] = make(chan interface{}, 256) // buffered channel
		netObj.recvChan[i] = make(chan interface{}, 256)

		// sender loop
		go func(to int) {
			for msg := range netObj.sendChan[to] {
				switch m := msg.(type) {
				case *ckks.Ciphertext:
					netObj.SendCiphertext(m, to)
				case int:
					netObj.SendInt(m, to)
				// add other cases (RData, Poly, etc.) as needed
				default:
					panic("unsupported message type")
				}
			}
		}(i)

		// receiver loop
		go func(from int) {
			for {
				// here you must know what to expect; e.g., loop:
				ct := netObj.ReceiveCiphertextBatch(netObj.cryptoParams, from)
				netObj.recvChan[from] <- ct
				// repeat or break based on protocol phases
			}
		}(i)
	}

	for i := 0; i < np; i++ {
		netObj.ctBuf[i] = make([]*ckks.Ciphertext, 0, netObj.ctThresh)
		netObj.ctMu[i] = &sync.Mutex{}
	}

	// Initialize the int‑buffer and mutex for each peer
	for i := 0; i < np; i++ {
		netObj.intBuf[i] = make([]uint64, 0, IntBatchSize)
		netObj.intMu[i] = &sync.Mutex{}
	}

	return netObj
}

// --- Buffered Ints ---

func (n *Network) SendInt(val, to int) {
	n.intMu[to].Lock()
	defer n.intMu[to].Unlock()
	n.intBuf[to] = append(n.intBuf[to], uint64(val))
	if len(n.intBuf[to]) >= IntBatchSize {
		n.flushIntBuf(to)
	}
}

func (n *Network) flushIntBuf(to int) {
	buf := n.intBuf[to]
	if len(buf) == 0 {
		return
	}
	n.SendIntVector(buf, to)
	n.intBuf[to] = buf[:0]
}

func (n *Network) FlushAllInts() {
	for to := range n.conns {
		n.intMu[to].Lock()
		n.flushIntBuf(to)
		n.intMu[to].Unlock()
	}
}

func (n *Network) SendIntVector(v []uint64, to int) {
	conn := n.conns[to]
	b := make([]byte, 8*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint64(b[i*8:(i*8)+8], x)
	}
	WriteFull(&conn, b)
	n.UpdateSenderLog(to, len(b))
}

func (n *Network) ReceiveInt(from int) int {
	conn := n.conns[from]
	buf := make([]byte, 8)
	ReadFull(&conn, buf)
	n.UpdateReceiverLog(from, 8)
	return int(binary.LittleEndian.Uint64(buf))
}

func (n *Network) ReceiveIntVector(nElem, from int) []uint64 {
	conn := n.conns[from]
	data := make([]byte, nElem*8)
	ReadFull(&conn, data)
	out := make([]uint64, nElem)
	for i := range out {
		out[i] = binary.LittleEndian.Uint64(data[i*8 : i*8+8])
	}
	n.UpdateReceiverLog(from, len(data))
	return out
}

// ReceiveRVec reads an RVec of length `length` from peer `from`
func (n *Network) ReceiveRVec(rtype mpc_core.RElem, length, from int) mpc_core.RVec {
	conn := n.conns[from]
	hdr := make([]byte, 4)
	ReadFull(&conn, hdr)
	sz := binary.LittleEndian.Uint32(hdr)
	buf := make([]byte, sz)
	ReadFull(&conn, buf)
	vec := mpc_core.InitRVec(rtype.Zero(), length)
	vec.UnmarshalBinary(buf)
	n.UpdateReceiverLog(from, 4+int(sz))
	return vec
}

// --- Polynomials ---

func (n *Network) SendPoly(poly *ring.Poly, to int) {
	conn := n.conns[to]
	data, _ := poly.MarshalBinary()
	hdr := make([]byte, 4)
	binary.LittleEndian.PutUint32(hdr, uint32(len(data)))
	WriteFull(&conn, hdr)
	WriteFull(&conn, data)
	n.UpdateSenderLog(to, len(hdr)+len(data))
}

func (n *Network) ReceivePoly(from int) *ring.Poly {
	conn := n.conns[from]
	hdr := make([]byte, 4)
	ReadFull(&conn, hdr)
	size := binary.LittleEndian.Uint32(hdr)
	data := make([]byte, size)
	ReadFull(&conn, data)
	poly := new(ring.Poly)
	poly.UnmarshalBinary(data)
	n.UpdateReceiverLog(from, len(hdr)+int(size))
	return poly
}

func (n *Network) SendPolyMat(mat [][]ring.Poly, to int) {
	conn := n.conns[to]
	sizes, data := MarshalPolyMat(mat)
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint64(hdr, uint64(len(sizes)))
	WriteFull(&conn, hdr)
	WriteFull(&conn, sizes)
	binary.LittleEndian.PutUint64(hdr, uint64(len(data)))
	WriteFull(&conn, hdr)
	WriteFull(&conn, data)
	n.UpdateSenderLog(to, 2*len(hdr)+len(sizes)+len(data))
}

func (n *Network) ReceivePolyMat(from int) [][]ring.Poly {
	conn := n.conns[from]
	hdr := make([]byte, 8)
	ReadFull(&conn, hdr)
	sz := binary.LittleEndian.Uint64(hdr)
	sizes := make([]byte, sz)
	ReadFull(&conn, sizes)
	ReadFull(&conn, hdr)
	dsz := binary.LittleEndian.Uint64(hdr)
	data := make([]byte, dsz)
	ReadFull(&conn, data)
	n.UpdateReceiverLog(from, 2*len(hdr)+int(sz)+int(dsz))
	return UnmarshalPolyMat(sizes, data)
}

// --- Ciphertexts ---

// SendCiphertext buffers ct; flushes once buffer reaches threshold.
func (n *Network) SendCiphertext(ct *ckks.Ciphertext, to int) {
	n.ctMu[to].Lock()
	defer n.ctMu[to].Unlock()

	n.ctBuf[to] = append(n.ctBuf[to], ct)
	if len(n.ctBuf[to]) >= n.ctThresh {
		n.flushCiphertexts(to)
	}
}

// flushCiphertexts marshals all buffered cts for peer 'to' and sends in one write.
func (n *Network) flushCiphertexts(to int) {
	bufList := n.ctBuf[to]
	if len(bufList) == 0 {
		return
	}

	// 1) count header
	count := uint32(len(bufList))
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, count)

	// 2) marshal each ciphertext: [4‑byte len][bytes]…
	var payload []byte
	for _, ct := range bufList {
		ctBytes, _ := ct.MarshalBinary()
		tmp := make([]byte, 4+len(ctBytes))
		binary.LittleEndian.PutUint32(tmp[:4], uint32(len(ctBytes)))
		copy(tmp[4:], ctBytes)
		payload = append(payload, tmp...)
	}

	// 3) write header + payload in one go
	conn := n.conns[to]
	WriteFull(&conn, header)
	WriteFull(&conn, payload)
	n.UpdateSenderLog(to, len(header)+len(payload))

	// 4) reset buffer
	n.ctBuf[to] = bufList[:0]
}

// FlushAllCiphertexts forces all leftover buffered cts out on the wire.
func (n *Network) FlushAllCiphertexts() {
	for to := range n.ctBuf {
		n.ctMu[to].Lock()
		n.flushCiphertexts(to)
		n.ctMu[to].Unlock()
	}
}

// ReceiveCiphertextBatch reads exactly one batch from 'from' and returns the slice.
func (n *Network) ReceiveCiphertextBatch(params *crypto.CryptoParams, from int) []*ckks.Ciphertext {
	conn := n.conns[from]

	// 1) read the count
	hdr := make([]byte, 4)
	ReadFull(&conn, hdr)
	count := int(binary.LittleEndian.Uint32(hdr))

	// 2) for each, read len + payload, unmarshal
	out := make([]*ckks.Ciphertext, count)
	for i := 0; i < count; i++ {
		// read the length
		ReadFull(&conn, hdr)
		ctLen := binary.LittleEndian.Uint32(hdr)

		// read the ciphertext bytes
		ctBytes := make([]byte, ctLen)
		ReadFull(&conn, ctBytes)

		// unmarshal
		ct := ckks.NewCiphertext(params.Params, 1, params.Params.MaxLevel(), params.Params.Scale())
		if err := ct.UnmarshalBinary(ctBytes); err != nil {
			panic(err)
		}
		out[i] = ct
	}

	// // 3) logging
	// totalBytes := 4 + count*(4) // 4 for count + 4 per-length header
	// for _, ct := range out {
	// 	totalBytes += int(ct.MarshalBinarySize()) // or track len(ctBytes)
	// }
	// n.UpdateReceiverLog(from, totalBytes)

	return out
}

func (n *Network) ReceiveCiphertext(params *crypto.CryptoParams, from int) *ckks.Ciphertext {
	conn := n.conns[from]

	// read length header
	hdr := make([]byte, 8)
	ReadFull(&conn, hdr)
	sz := binary.LittleEndian.Uint64(hdr)

	// read body
	data := make([]byte, sz)
	ReadFull(&conn, data)

	n.UpdateReceiverLog(from, 8+int(sz))

	// unmarshal
	ct := ckks.NewCiphertext(params.Params, 1, params.Params.MaxLevel(), params.Params.Scale())
	if err := ct.UnmarshalBinary(data); err != nil {
		panic(err)
	}
	return ct
}

func (n *Network) SendCipherMatrix(cm crypto.CipherMatrix, to int) {
	conn := n.conns[to]
	sbytes, cmbytes := crypto.MarshalCM(cm)
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint64(hdr, uint64(len(sbytes)))
	WriteFull(&conn, hdr)
	WriteFull(&conn, sbytes)
	binary.LittleEndian.PutUint64(hdr, uint64(len(cmbytes)))
	WriteFull(&conn, hdr)
	WriteFull(&conn, cmbytes)
	n.UpdateSenderLog(to, 2*len(hdr)+len(sbytes)+len(cmbytes))
}

func (n *Network) ReceiveCipherMatrix(params *crypto.CryptoParams, nv, nct, from int) crypto.CipherMatrix {
	cm := make(crypto.CipherMatrix, nv)
	for i := 0; i < nv; i++ {
		cm[i] = n.ReceiveCipherVector(params, nct, from)
	}
	return cm
}

func (n *Network) SendCipherVector(cv crypto.CipherVector, to int) {
	conn := n.conns[to]
	sbytes, cvbytes := MarshalCV(cv)
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint64(hdr, uint64(len(sbytes)))
	WriteFull(&conn, hdr)
	WriteFull(&conn, sbytes)
	binary.LittleEndian.PutUint64(hdr, uint64(len(cvbytes)))
	WriteFull(&conn, hdr)
	WriteFull(&conn, cvbytes)
	n.UpdateSenderLog(to, 2*len(hdr)+len(sbytes)+len(cvbytes))
}

func (n *Network) ReceiveCipherVector(params *crypto.CryptoParams, nct, from int) crypto.CipherVector {
	conn := n.conns[from]
	hdr := make([]byte, 8)
	ReadFull(&conn, hdr)
	ssz := binary.LittleEndian.Uint64(hdr)
	sdata := make([]byte, ssz)
	ReadFull(&conn, sdata)
	ReadFull(&conn, hdr)
	csz := binary.LittleEndian.Uint64(hdr)
	cdata := make([]byte, csz)
	ReadFull(&conn, cdata)
	n.UpdateReceiverLog(from, 2*len(hdr)+int(ssz)+int(csz))
	return UnmarshalCV(params, nct, sdata, cdata)
}

// SendRData / ReceiveRMat / ReceiveRElem

func (n *Network) SendRData(data interface{}, to int) {
	conn := n.conns[to]
	buf := MarshalRData(data)
	var hdr []byte
	if _, isElem := data.(mpc_core.RElem); !isElem {
		hdr = make([]byte, 4)
		binary.LittleEndian.PutUint32(hdr, uint32(len(buf)))
		WriteFull(&conn, hdr)
	}
	WriteFull(&conn, buf)
	n.UpdateSenderLog(to, len(hdr)+len(buf))
}

func (n *Network) ReceiveRMat(rtype mpc_core.RElem, nrows, ncols, from int) mpc_core.RMat {
	conn := n.conns[from]
	hdr := make([]byte, 4)
	ReadFull(&conn, hdr)
	sz := binary.LittleEndian.Uint32(hdr)
	data := make([]byte, sz)
	ReadFull(&conn, data)
	mat := mpc_core.InitRMat(rtype.Zero(), nrows, ncols)
	mat.UnmarshalBinary(data)
	n.UpdateReceiverLog(from, 4+int(sz))
	return mat
}

func (n *Network) ReceiveRElem(rtype mpc_core.RElem, from int) mpc_core.RElem {
	conn := n.conns[from]
	buf := make([]byte, rtype.NumBytes())
	ReadFull(&conn, buf)
	n.UpdateReceiverLog(from, len(buf))
	return rtype.FromBytes(buf)
}

// Utility read/write

func WriteFull(conn *net.Conn, buf []byte) {
	offs, rem := 0, len(buf)
	for rem > 0 {
		w, err := (*conn).Write(buf[offs:])
		if err != nil {
			panic(err)
		}
		offs += w
		rem -= w
	}
}

func ReadFull(conn *net.Conn, buf []byte) {
	offs, rem := 0, len(buf)
	for rem > 0 {
		r, err := (*conn).Read(buf[offs:])
		if err != nil {
			panic(err)
		}
		offs += r
		rem -= r
	}
}

// Connection helpers

func OpenChannel(ip, port string) (net.Conn, net.Listener) {
	addr := ip + ":" + port
	l, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	c, err := l.Accept()
	if err != nil {
		panic(err)
	}
	return c, l
}

func Connect(ip, port string) net.Conn {
	addr := ip + ":" + port
	var c net.Conn
	var err error
	for i := 0; i < 5; i++ {
		c, err = net.Dial("tcp", addr)
		if err == nil {
			return c
		}
		time.Sleep(5 * time.Second)
	}
	panic(fmt.Sprintf("failed to connect to %s", addr))
}

func SaveBytesToFile(b []byte, filename string) {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.Write(b)
	f.Sync()
}

func (n *Network) CloseAll() {
	for _, c := range n.conns {
		c.Close()
	}
	for _, l := range n.listeners {
		l.Close()
	}
}

func (n *Network) GetConn(to int, threadNum int) net.Conn { return n.conns[to] }
func (n *Network) SetPid(p int)                           { n.pid = p }
func (n *Network) GetPid() int                            { return n.pid }
func (n *Network) SetHubPid(p int)                        { n.hubPid = p }
func (n *Network) GetHubPid() int                         { return n.hubPid }
func (n *Network) SetNParty(np int)                       { n.NumParties = np }
func (n *Network) GetNParty() int                         { return n.NumParties }
func (n *Network) GetCRPGen() *ring.UniformSampler        { return n.crpGen }
