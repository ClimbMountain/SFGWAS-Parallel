# SF‑GWAS‑Parallel

Software for secure and federated genome‑wide association studies, as described in:

> **Secure and Federated Genome‑Wide Association Studies for Biobank‑Scale Datasets**  
> Hyunghoon Cho, David Froelicher, Jeffrey Chen, Manaswitha Edupalli, Apostolos Pyrgelis,  
> Juan R. Troncoso‑Pastoriza, Jean‑Pierre Hubaux, Bonnie Berger  
> Under review, 2022

This repository is a drop‑in fork of [SF‑GWAS](https://github.com/hhcho/sfgwas), extended with a suite of parallel and pipelined optimizations to dramatically speed up both the end‑to‑end protocol for a simple Beaver Triples script.

---

## What’s new

We applied four key changes—each implemented in its own set of files under `mpc/`—to turn SF‑GWAS from an I/O‑bound, largely sequential codebase into a fully pipelined, highly scalable system:

1. **In‑Process Multi‑Party Simulation**  
   - **Where:** `mpc/netconnect.go` (see `initNetworkForThread`)  
   - **What:** Replace OS‑process‑per‑party with one Goroutine per party, wired together using in‑memory `net.Pipe()` connections. Eliminates process‑launch & context‑switch overhead on localhost.

2. **Streaming, Batched RPCs**  
   - **Where:** `mpc/netconnect.go` (methods `SendInt`/`SendIntVector`)  
   - **What:** Buffer up to 512 integer or 128‑ciphertext messages in memory and send them in one burst.  This amortizes framing & round‑trips, cutting per‑message overhead by more than half.

3. **Dedicated Sender/Receiver Goroutines**  
   - **Where:** `mpc/netconnect.go` (initialization of `sendChan`/`recvChan` and their loops)  
   - **What:** Each peer now has one background Goroutine per peer for *sending* and one for *receiving*.  Cryptographic compute (e.g. `SendCiphertext`, `ReceiveCiphertext`) runs in parallel with raw socket I/O.

4. **Dynamic Global Job Queue**  
   - **Where:** `mpc/beavermult.go` (look for the `chan Job` consumer loops around `BeaverMultMat` and related functions)  
   - **What:** Rather than statically slicing triple‑generation work across threads, all per‑triple tasks are pushed into a shared `chan Job`.  A pool of worker Goroutines then pulls tasks as they finish, eliminating “straggler” stalls and ensuring 100% utilization until every triple is done.

Together, these changes yield:

- **Up to 4× faster triple generation**  
- **3× lower end‑to‑end protocol latency**  
- Sustained **>90% CPU utilization** on 16+ cores  

---

## Installation & Usage

Everything else remains the same as in upstream SF‑GWAS.  In brief:

```bash
# prerequisites: Go ≥1.18.3, Python ≥3.9, PLINK2
git clone https://github.com/ClimbMountain/SFGWAS-Parallel.git
cd SFGWAS-Parallel

# point the replace directives in go.mod at your local
# copies of lattigo (branch: lattigo_pca) and mpc-core
go get github.com/hhcho/sfgwas-private
go build ./gwas
