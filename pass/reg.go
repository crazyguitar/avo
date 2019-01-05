package pass

import (
	"errors"

	"github.com/mmcloughlin/avo"
	"github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

// Liveness computes register liveness.
func Liveness(fn *avo.Function) error {
	// Note this implementation is initially naive so as to be "obviously correct".
	// There are a well-known optimizations we can apply if necessary.

	is := fn.Instructions()

	// Process instructions in reverse: poor approximation to topological sort.
	// TODO(mbm): process instructions in topological sort order
	for l, r := 0, len(is)-1; l < r; l, r = l+1, r-1 {
		is[l], is[r] = is[r], is[l]
	}

	// Initialize.
	for _, i := range is {
		i.LiveIn = reg.NewSetFromSlice(i.InputRegisters())
		i.LiveOut = reg.NewEmptySet()
	}

	// Iterative dataflow analysis.
	for {
		changes := false

		for _, i := range is {
			// out[n] = UNION[s IN succ[n]] in[s]
			nout := len(i.LiveOut)
			for _, s := range i.Succ {
				if s == nil {
					continue
				}
				i.LiveOut.Update(s.LiveIn)
			}
			if len(i.LiveOut) != nout {
				changes = true
			}

			// in[n] = use[n] UNION (out[n] - def[n])
			nin := len(i.LiveIn)
			def := reg.NewSetFromSlice(i.OutputRegisters())
			i.LiveIn.Update(i.LiveOut.Difference(def))
			for r := range i.LiveOut {
				if _, found := def[r]; !found {
					i.LiveIn.Add(r)
				}
			}
			if len(i.LiveIn) != nin {
				changes = true
			}
		}

		if !changes {
			break
		}
	}

	return nil
}

// AllocateRegisters performs register allocation.
func AllocateRegisters(fn *avo.Function) error {
	// Populate allocators (one per kind).
	as := map[reg.Kind]*Allocator{}
	for _, i := range fn.Instructions() {
		for _, r := range i.Registers() {
			k := r.Kind()
			if _, found := as[k]; !found {
				a, err := NewAllocatorForKind(k)
				if err != nil {
					return err
				}
				as[k] = a
			}
			as[k].Add(r)
		}
	}

	// Record register interferences.
	for _, i := range fn.Instructions() {
		for _, d := range i.OutputRegisters() {
			k := d.Kind()
			out := i.LiveOut.OfKind(k)
			out.Discard(d)
			as[k].AddInterferenceSet(d, out)
		}
	}

	// Execute register allocation.
	fn.Allocation = reg.NewEmptyAllocation()
	for _, a := range as {
		al, err := a.Allocate()
		if err != nil {
			return err
		}
		if err := fn.Allocation.Merge(al); err != nil {
			return err
		}
	}

	return nil
}

// BindRegisters applies the result of register allocation, replacing all virtual registers with their assigned physical registers.
func BindRegisters(fn *avo.Function) error {
	for _, i := range fn.Instructions() {
		for idx := range i.Operands {
			i.Operands[idx] = operand.ApplyAllocation(i.Operands[idx], fn.Allocation)
		}
	}
	return nil
}

// VerifyAllocation performs sanity checks following register allocation.
func VerifyAllocation(fn *avo.Function) error {
	// All registers should be physical.
	for _, i := range fn.Instructions() {
		for _, r := range i.Registers() {
			if reg.ToPhysical(r) == nil {
				return errors.New("non physical register found")
			}
		}
	}

	return nil
}
