// Package pass implements processing passes on avo Files.
package pass

import (
	"io"

	"github.com/mmcloughlin/avo"
	"github.com/mmcloughlin/avo/printer"
)

// Compile pass compiles an avo file. Upon successful completion the avo file
// may be printed to Go assembly.
var Compile = Concat(
	FunctionPass(LabelTarget),
	FunctionPass(CFG),
	FunctionPass(Liveness),
	FunctionPass(AllocateRegisters),
	FunctionPass(BindRegisters),
	FunctionPass(VerifyAllocation),
	Func(IncludeTextFlagHeader),
)

// Interface for a processing pass.
type Interface interface {
	Execute(*avo.File) error
}

// Func adapts a function to the pass Interface.
type Func func(*avo.File) error

// Execute calls p.
func (p Func) Execute(f *avo.File) error {
	return p(f)
}

// FunctionPass is a convenience for implementing a full file pass with a
// function that operates on each avo Function independently.
type FunctionPass func(*avo.Function) error

// Execute calls p on every function in the file. Exits on the first error.
func (p FunctionPass) Execute(f *avo.File) error {
	for _, fn := range f.Functions() {
		if err := p(fn); err != nil {
			return err
		}
	}
	return nil
}

// Concat returns a pass that executes the given passes in order, stopping on the first error.
func Concat(passes ...Interface) Interface {
	return Func(func(f *avo.File) error {
		for _, p := range passes {
			if err := p.Execute(f); err != nil {
				return err
			}
		}
		return nil
	})
}

// Output pass prints a file.
type Output struct {
	Writer  io.WriteCloser
	Printer printer.Printer
}

// Execute prints f with the configured Printer and writes output to Writer.
func (o *Output) Execute(f *avo.File) error {
	b, err := o.Printer.Print(f)
	if err != nil {
		return err
	}
	if _, err = o.Writer.Write(b); err != nil {
		return err
	}
	return o.Writer.Close()
}
