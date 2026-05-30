//go:build ignore

// generate.go creates test .wasm fixtures for the plugin package.
// Run: go run ./internal/plugin/testdata/generate.go
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	dir := filepath.Dir(callerFile())

	// pass.wasm — Memory ABI plugin that always passes
	writeFile(filepath.Join(dir, "pass.wasm"), buildPassModule())

	// fail.wasm — Memory ABI plugin that always fails
	writeFile(filepath.Join(dir, "fail.wasm"), buildFailModule())

	// crash.wasm — Memory ABI plugin that hits unreachable
	writeFile(filepath.Join(dir, "crash.wasm"), buildCrashModule())

	// timeout.wasm — Memory ABI plugin with infinite loop
	writeFile(filepath.Join(dir, "timeout.wasm"), buildTimeoutModule())

	fmt.Println("Generated all test .wasm fixtures")
}

func callerFile() string {
	_, file, _, _ := runtime.Caller(0)
	return file
}

func writeFile(path string, data []byte) {
	if err := os.WriteFile(path, data, 0644); err != nil {
		panic(fmt.Sprintf("writing %s: %v", path, err))
	}
	fmt.Printf("  wrote %s (%d bytes)\n", filepath.Base(path), len(data))
}

// === Wasm binary encoding helpers ===

const (
	wasmMagic   = 0x0061736D
	wasmVersion = 0x00000001
)

func wasmHeader() []byte {
	// Wasm binary format: magic \0asm followed by version 1 (little-endian u32)
	return []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
}

func section(id byte, content []byte) []byte {
	out := []byte{id}
	out = append(out, uleb128(uint32(len(content)))...)
	out = append(out, content...)
	return out
}

func uleb128(v uint32) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

func sleb128(v int32) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v == 0 && (b&0x40) == 0 {
			buf = append(buf, b)
			break
		}
		if v == -1 && (b&0x40) != 0 {
			buf = append(buf, b)
			break
		}
		b |= 0x80
		buf = append(buf, b)
	}
	return buf
}

func vecLen(n int) []byte {
	return uleb128(uint32(n))
}

func name(s string) []byte {
	b := uleb128(uint32(len(s)))
	b = append(b, []byte(s)...)
	return b
}

// buildPassModule creates a memory-ABI module that returns a passing JSON result.
// Exports: memory, smokesig_alloc, smokesig_dealloc, evaluate
func buildPassModule() []byte {
	passJSON := `{"pass":true,"message":"ok","details":[{"type":"test","expected":"pass","actual":"pass","pass":true}],"error":null}`
	return buildMemoryABIModule(passJSON)
}

// buildFailModule creates a memory-ABI module that returns a failing JSON result.
func buildFailModule() []byte {
	failJSON := `{"pass":false,"message":"assertion failed","details":[{"type":"test","expected":"pass","actual":"fail","pass":false}],"error":null}`
	return buildMemoryABIModule(failJSON)
}

// buildMemoryABIModule creates a Wasm module with memory ABI that returns a static JSON string.
// The module has:
//   - 1 page of memory (exported as "memory")
//   - Static data at offset 1024
//   - A bump allocator starting at offset 4096
//   - smokesig_alloc(size) -> ptr
//   - smokesig_dealloc(ptr, len) -> void
//   - evaluate(ptr, len) -> i64 ((result_ptr << 32) | result_len)
func buildMemoryABIModule(jsonResult string) []byte {
	jsonBytes := []byte(jsonResult)
	jsonLen := len(jsonBytes)
	dataOffset := 1024

	mod := wasmHeader()

	// Type section (section 1): function signatures
	// Type 0: (i32) -> (i32)      — smokesig_alloc
	// Type 1: (i32, i32) -> ()    — smokesig_dealloc
	// Type 2: (i32, i32) -> (i64) — evaluate
	typeContent := vecLen(3)
	// Type 0: (i32) -> (i32)
	typeContent = append(typeContent, 0x60)      // func
	typeContent = append(typeContent, vecLen(1)...) // 1 param
	typeContent = append(typeContent, 0x7F)      // i32
	typeContent = append(typeContent, vecLen(1)...) // 1 result
	typeContent = append(typeContent, 0x7F)      // i32
	// Type 1: (i32, i32) -> ()
	typeContent = append(typeContent, 0x60)      // func
	typeContent = append(typeContent, vecLen(2)...) // 2 params
	typeContent = append(typeContent, 0x7F, 0x7F) // i32, i32
	typeContent = append(typeContent, vecLen(0)...) // 0 results
	// Type 2: (i32, i32) -> (i64)
	typeContent = append(typeContent, 0x60)      // func
	typeContent = append(typeContent, vecLen(2)...) // 2 params
	typeContent = append(typeContent, 0x7F, 0x7F) // i32, i32
	typeContent = append(typeContent, vecLen(1)...) // 1 result
	typeContent = append(typeContent, 0x7E)      // i64
	mod = append(mod, section(1, typeContent)...)

	// Function section (section 3): declares function type indices
	funcContent := vecLen(3) // 3 functions
	funcContent = append(funcContent, 0x00) // func 0 -> type 0 (alloc)
	funcContent = append(funcContent, 0x01) // func 1 -> type 1 (dealloc)
	funcContent = append(funcContent, 0x02) // func 2 -> type 2 (evaluate)
	mod = append(mod, section(3, funcContent)...)

	// Memory section (section 5): 1 memory with 1 initial page
	memContent := vecLen(1) // 1 memory
	memContent = append(memContent, 0x00) // no maximum
	memContent = append(memContent, uleb128(1)...) // 1 initial page
	mod = append(mod, section(5, memContent)...)

	// Global section (section 6): mutable i32 for bump allocator
	globalContent := vecLen(1) // 1 global
	globalContent = append(globalContent, 0x7F) // i32
	globalContent = append(globalContent, 0x01) // mutable
	// init expr: i32.const 4096, end
	globalContent = append(globalContent, 0x41) // i32.const
	globalContent = append(globalContent, uleb128(4096)...)
	globalContent = append(globalContent, 0x0B) // end
	mod = append(mod, section(6, globalContent)...)

	// Export section (section 7): memory, smokesig_alloc, smokesig_dealloc, evaluate
	exportContent := vecLen(4) // 4 exports
	// Export "memory" -> memory 0
	exportContent = append(exportContent, name("memory")...)
	exportContent = append(exportContent, 0x02) // memory
	exportContent = append(exportContent, 0x00) // index 0
	// Export "smokesig_alloc" -> func 0
	exportContent = append(exportContent, name("smokesig_alloc")...)
	exportContent = append(exportContent, 0x00) // func
	exportContent = append(exportContent, 0x00) // index 0
	// Export "smokesig_dealloc" -> func 1
	exportContent = append(exportContent, name("smokesig_dealloc")...)
	exportContent = append(exportContent, 0x00) // func
	exportContent = append(exportContent, 0x01) // index 1
	// Export "evaluate" -> func 2
	exportContent = append(exportContent, name("evaluate")...)
	exportContent = append(exportContent, 0x00) // func
	exportContent = append(exportContent, 0x02) // index 2
	mod = append(mod, section(7, exportContent)...)

	// Code section (section 10): function bodies
	var bodies []byte

	// Function 0: smokesig_alloc(size i32) -> i32
	// Returns current bump pointer, advances by size
	{
		var body []byte
		body = append(body, vecLen(0)...) // 0 locals
		// global.get $bump
		body = append(body, 0x23, 0x00)
		// global.get $bump
		body = append(body, 0x23, 0x00)
		// local.get $size
		body = append(body, 0x20, 0x00)
		// i32.add
		body = append(body, 0x6A)
		// global.set $bump
		body = append(body, 0x24, 0x00)
		// end
		body = append(body, 0x0B)
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}

	// Function 1: smokesig_dealloc(ptr i32, len i32) -> void
	// No-op (bump allocator)
	{
		var body []byte
		body = append(body, vecLen(0)...) // 0 locals
		body = append(body, 0x0B)         // end
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}

	// Function 2: evaluate(ptr i32, len i32) -> i64
	// Returns (dataOffset << 32) | jsonLen as i64
	{
		resultPacked := (uint64(dataOffset) << 32) | uint64(jsonLen)
		var body []byte
		body = append(body, vecLen(0)...) // 0 locals
		// i64.const resultPacked
		body = append(body, 0x42) // i64.const
		body = append(body, sleb128_64(int64(resultPacked))...)
		// end
		body = append(body, 0x0B)
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}

	codeContent := vecLen(3) // 3 function bodies
	codeContent = append(codeContent, bodies...)
	mod = append(mod, section(10, codeContent)...)

	// Data section (section 11): store JSON at offset 1024
	dataContent := vecLen(1)   // 1 data segment
	dataContent = append(dataContent, 0x00) // active, memory 0
	// init expr: i32.const dataOffset, end
	dataContent = append(dataContent, 0x41) // i32.const
	dataContent = append(dataContent, uleb128(uint32(dataOffset))...)
	dataContent = append(dataContent, 0x0B) // end
	dataContent = append(dataContent, uleb128(uint32(jsonLen))...)
	dataContent = append(dataContent, jsonBytes...)
	mod = append(mod, section(11, dataContent)...)

	return mod
}

func sleb128_64(v int64) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v == 0 && (b&0x40) == 0 {
			buf = append(buf, b)
			break
		}
		if v == -1 && (b&0x40) != 0 {
			buf = append(buf, b)
			break
		}
		b |= 0x80
		buf = append(buf, b)
	}
	return buf
}

// buildCrashModule creates a module whose evaluate function hits unreachable.
func buildCrashModule() []byte {
	mod := wasmHeader()

	// Type section
	typeContent := vecLen(3)
	// Type 0: (i32) -> (i32) — alloc
	typeContent = append(typeContent, 0x60, 0x01, 0x7F, 0x01, 0x7F)
	// Type 1: (i32, i32) -> () — dealloc
	typeContent = append(typeContent, 0x60, 0x02, 0x7F, 0x7F, 0x00)
	// Type 2: (i32, i32) -> (i64) — evaluate
	typeContent = append(typeContent, 0x60, 0x02, 0x7F, 0x7F, 0x01, 0x7E)
	mod = append(mod, section(1, typeContent)...)

	// Function section
	funcContent := vecLen(3)
	funcContent = append(funcContent, 0x00, 0x01, 0x02)
	mod = append(mod, section(3, funcContent)...)

	// Memory section
	memContent := vecLen(1)
	memContent = append(memContent, 0x00)
	memContent = append(memContent, uleb128(1)...)
	mod = append(mod, section(5, memContent)...)

	// Global section: bump allocator
	globalContent := vecLen(1)
	globalContent = append(globalContent, 0x7F, 0x01) // i32, mutable
	globalContent = append(globalContent, 0x41)
	globalContent = append(globalContent, uleb128(4096)...)
	globalContent = append(globalContent, 0x0B)
	mod = append(mod, section(6, globalContent)...)

	// Export section
	exportContent := vecLen(4)
	exportContent = append(exportContent, name("memory")...)
	exportContent = append(exportContent, 0x02, 0x00)
	exportContent = append(exportContent, name("smokesig_alloc")...)
	exportContent = append(exportContent, 0x00, 0x00)
	exportContent = append(exportContent, name("smokesig_dealloc")...)
	exportContent = append(exportContent, 0x00, 0x01)
	exportContent = append(exportContent, name("evaluate")...)
	exportContent = append(exportContent, 0x00, 0x02)
	mod = append(mod, section(7, exportContent)...)

	// Code section
	var bodies []byte
	// alloc: bump allocator (same as pass)
	{
		var body []byte
		body = append(body, vecLen(0)...)
		body = append(body, 0x23, 0x00) // global.get
		body = append(body, 0x23, 0x00) // global.get
		body = append(body, 0x20, 0x00) // local.get
		body = append(body, 0x6A)       // i32.add
		body = append(body, 0x24, 0x00) // global.set
		body = append(body, 0x0B)       // end
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}
	// dealloc: no-op
	{
		var body []byte
		body = append(body, vecLen(0)...)
		body = append(body, 0x0B)
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}
	// evaluate: unreachable
	{
		var body []byte
		body = append(body, vecLen(0)...)
		body = append(body, 0x00) // unreachable
		body = append(body, 0x0B) // end
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}

	codeContent := vecLen(3)
	codeContent = append(codeContent, bodies...)
	mod = append(mod, section(10, codeContent)...)

	return mod
}

// buildTimeoutModule creates a module whose evaluate function loops forever.
func buildTimeoutModule() []byte {
	mod := wasmHeader()

	// Type section
	typeContent := vecLen(3)
	typeContent = append(typeContent, 0x60, 0x01, 0x7F, 0x01, 0x7F) // (i32)->i32
	typeContent = append(typeContent, 0x60, 0x02, 0x7F, 0x7F, 0x00) // (i32,i32)->void
	typeContent = append(typeContent, 0x60, 0x02, 0x7F, 0x7F, 0x01, 0x7E) // (i32,i32)->i64
	mod = append(mod, section(1, typeContent)...)

	// Function section
	funcContent := vecLen(3)
	funcContent = append(funcContent, 0x00, 0x01, 0x02)
	mod = append(mod, section(3, funcContent)...)

	// Memory section
	memContent := vecLen(1)
	memContent = append(memContent, 0x00)
	memContent = append(memContent, uleb128(1)...)
	mod = append(mod, section(5, memContent)...)

	// Global section
	globalContent := vecLen(1)
	globalContent = append(globalContent, 0x7F, 0x01)
	globalContent = append(globalContent, 0x41)
	globalContent = append(globalContent, uleb128(4096)...)
	globalContent = append(globalContent, 0x0B)
	mod = append(mod, section(6, globalContent)...)

	// Export section
	exportContent := vecLen(4)
	exportContent = append(exportContent, name("memory")...)
	exportContent = append(exportContent, 0x02, 0x00)
	exportContent = append(exportContent, name("smokesig_alloc")...)
	exportContent = append(exportContent, 0x00, 0x00)
	exportContent = append(exportContent, name("smokesig_dealloc")...)
	exportContent = append(exportContent, 0x00, 0x01)
	exportContent = append(exportContent, name("evaluate")...)
	exportContent = append(exportContent, 0x00, 0x02)
	mod = append(mod, section(7, exportContent)...)

	// Code section
	var bodies []byte
	// alloc
	{
		var body []byte
		body = append(body, vecLen(0)...)
		body = append(body, 0x23, 0x00, 0x23, 0x00, 0x20, 0x00, 0x6A, 0x24, 0x00, 0x0B)
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}
	// dealloc
	{
		var body []byte
		body = append(body, vecLen(0)...)
		body = append(body, 0x0B)
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}
	// evaluate: infinite loop
	// block -> loop -> br 0 -> end -> end
	{
		var body []byte
		body = append(body, vecLen(0)...)
		// loop (no result)
		body = append(body, 0x03, 0x40) // loop, void block type
		// br 0 (branch to loop start)
		body = append(body, 0x0C, 0x00)
		// end loop
		body = append(body, 0x0B)
		// To satisfy i64 return, we need unreachable after loop (loop never exits)
		body = append(body, 0x00) // unreachable (never reached)
		// end function
		body = append(body, 0x0B)
		bodies = append(bodies, uleb128(uint32(len(body)))...)
		bodies = append(bodies, body...)
	}

	codeContent := vecLen(3)
	codeContent = append(codeContent, bodies...)
	mod = append(mod, section(10, codeContent)...)

	_ = math.MaxFloat64 // force math import for potential use

	return mod
}
