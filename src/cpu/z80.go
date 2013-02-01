package cpu

import (
	"errors"
	"fmt"
	"log"
	"mmu"
	"types"
	"utils"
)

const PREFIX = "CPU:"

//flags
const (
	_ = iota
	C
	H
	N
	Z
)

//hz
const CLOCK_RATE int = 4194304

type Registers struct {
	A byte
	B byte
	C byte
	D byte
	E byte
	H byte
	L byte
	F byte // Flags Register
}

func (r Registers) String() string {
	formatByte := func(b byte) string {
		if b < 0x10 {
			return fmt.Sprintf("0x0%X", b)
		}
		return fmt.Sprintf("0x%X", b)
	}
	return fmt.Sprintf("[A: %s | B: %s | C: %s | D: %s | E: %s | H: %s | L: %s]", formatByte(r.A), formatByte(r.B), formatByte(r.C), formatByte(r.D), formatByte(r.E), formatByte(r.H), formatByte(r.L))
}

//See ZILOG z80 cpu manual p.80  (http://www.zilog.com/docs/z80/um0080.pdf)
type Clock struct {
	M int
	t int
}

func (c *Clock) T() int {
	return c.M * 4
}

func (c *Clock) Reset() {
	c.M, c.t = 0, 0
}

func (c *Clock) String() string {
	return fmt.Sprintf("[M: %X, T: %X]", c.M, c.t)
}

type Z80 struct {
	PC                 types.Word // Program Counter
	SP                 types.Word // Stack Pointer
	R                  Registers
	Running            bool
	InterruptsEnabled  bool
	CurrentInstruction Instruction
	MachineCycles      Clock
	LastInstrCycle     Clock
	mmu                mmu.MemoryMappedUnit
	PCJumped           bool
}

func NewCPU() *Z80 {
	cpu := new(Z80)
	cpu.Reset()
	return cpu
}

func (cpu *Z80) LinkMMU(m mmu.MemoryMappedUnit) {
	cpu.mmu = m
	log.Println(PREFIX, "Linked CPU to MMU")
}

func (cpu *Z80) Validate() error {
	if cpu.mmu == nil {
		return errors.New("No MMU linked to CPU")
	}
	return nil
}

func (cpu *Z80) Reset() {
	cpu.PC = 0
	cpu.SP = 0
	cpu.R.A = 0
	cpu.R.B = 0
	cpu.R.C = 0
	cpu.R.D = 0
	cpu.R.E = 0
	cpu.R.F = 0
	cpu.R.H = 0
	cpu.R.L = 0
	cpu.CurrentInstruction, _ = cpu.Decode(0x00)
	cpu.InterruptsEnabled = true
	cpu.Running = true
	cpu.MachineCycles.Reset()
	cpu.LastInstrCycle.Reset()
	cpu.PCJumped = false
}

func (cpu *Z80) FlagsString() string {
	var flags string = ""
	var minus string = "-"

	if cpu.IsFlagSet(Z) {
		flags += "Z"
	} else {
		flags += minus
	}

	if cpu.IsFlagSet(N) {
		flags += "N"
	} else {
		flags += minus
	}

	if cpu.IsFlagSet(H) {
		flags += "H"
	} else {
		flags += minus
	}

	if cpu.IsFlagSet(C) {
		flags += "C"
	} else {
		flags += minus
	}
	return flags
}

func (cpu *Z80) String() string {
	return fmt.Sprintf("\nZ80 CPU\n") +
		fmt.Sprintf("--------------------------------------------------------\n") +
		fmt.Sprintf("\tInstruction = %v\n", cpu.CurrentInstruction) +
		fmt.Sprintf("\tPC		= %s\n", cpu.PC) +
		fmt.Sprintf("\tSP		= %s\n", cpu.SP) +
		fmt.Sprintf("\tINTS?		= %v\n", cpu.InterruptsEnabled) +
		fmt.Sprintf("\tLast Cycle	= %v\n", cpu.LastInstrCycle.String()) +
		fmt.Sprintf("\tMachine Cycles	= %v\n", cpu.MachineCycles.String()) +
		fmt.Sprintf("\tFlags		= %v\n", cpu.FlagsString()) +
		fmt.Sprintf("\n\tRegisters\n") +
		fmt.Sprintf("\tA:%X\tB:%X\tC:%X\tD:%X\n\tE:%X\tH:%X\tL:%X", cpu.R.A, cpu.R.B, cpu.R.C, cpu.R.D, cpu.R.E, cpu.R.H, cpu.R.L) +
		fmt.Sprintf("\n--------------------------------------------------------\n\n")
}

func (cpu *Z80) ResetFlag(flag int) {
	switch flag {
	case Z:
		cpu.R.F = cpu.R.F &^ 0x80
	case N:
		cpu.R.F = cpu.R.F &^ 0x40
	case H:
		cpu.R.F = cpu.R.F &^ 0x20
	case C:
		cpu.R.F = cpu.R.F &^ 0x10
	default:
		log.Fatalf(PREFIX+" Unknown flag %c", flag)
	}
}

func (cpu *Z80) SetFlag(flag int) {
	switch flag {
	case Z:
		if !cpu.IsFlagSet(Z) {
			cpu.R.F = cpu.R.F ^ 0x80
		}
	case N:
		if !cpu.IsFlagSet(N) {
			cpu.R.F = cpu.R.F ^ 0x40
		}
	case H:
		if !cpu.IsFlagSet(H) {
			cpu.R.F = cpu.R.F ^ 0x20
		}
	case C:
		if !cpu.IsFlagSet(C) {
			cpu.R.F = cpu.R.F ^ 0x10
		}
	default:
		log.Fatalf(PREFIX+" Unknown flag %c", flag)
	}
}

func (cpu *Z80) IsFlagSet(flag int) bool {
	switch flag {
	case Z:
		return cpu.R.F&0x80 == 0x80
	case N:
		return cpu.R.F&0x40 == 0x40
	case H:
		return cpu.R.F&0x20 == 0x20
	case C:
		return cpu.R.F&0x10 == 0x10
	default:
		log.Fatalf(PREFIX+" Unknown flag %c", flag)
	}
	return false
}

func (cpu *Z80) IncrementPC(by int) {
	cpu.PC += types.Word(by)
}

func (cpu *Z80) DispatchCB(Opcode byte) {
	switch Opcode {
	case 0x00: //RLC B
		cpu.Rlc_r(&cpu.R.B)
	case 0x01: //RLC C
		cpu.Rlc_r(&cpu.R.C)
	case 0x02: //RLC D
		cpu.Rlc_r(&cpu.R.D)
	case 0x03: //RLC E
		cpu.Rlc_r(&cpu.R.E)
	case 0x04: //RLC H
		cpu.Rlc_r(&cpu.R.H)
	case 0x05: //RLC L
		cpu.Rlc_r(&cpu.R.L)
	case 0x06: //RLC (HL)
		cpu.Rlc_hl()
	case 0x07: //RLC A
		cpu.Rlc_r(&cpu.R.A)
	case 0x08: //RRC B
		cpu.Rrc_r(&cpu.R.B)
	case 0x09: //RRC C
		cpu.Rrc_r(&cpu.R.C)
	case 0x0A: //RRC D
		cpu.Rrc_r(&cpu.R.D)
	case 0x0B: //RRC E
		cpu.Rrc_r(&cpu.R.E)
	case 0x0C: //RRC H
		cpu.Rrc_r(&cpu.R.H)
	case 0x0D: //RRC L
		cpu.Rrc_r(&cpu.R.L)
	case 0x0E: //RRC (HL)
		cpu.Rrc_hl()
	case 0x0F: //RRC A
		cpu.Rrc_r(&cpu.R.A)
	case 0x10: //RL B
		cpu.Rl_r(&cpu.R.B)
	case 0x11: //RL C
		cpu.Rl_r(&cpu.R.C)
	case 0x12: //RL D
		cpu.Rl_r(&cpu.R.D)
	case 0x13: //RL E
		cpu.Rl_r(&cpu.R.E)
	case 0x14: //RL H
		cpu.Rl_r(&cpu.R.H)
	case 0x15: //RL L
		cpu.Rl_r(&cpu.R.L)
	case 0x16: //RL (HL)
		cpu.Rl_hl()
	case 0x17: //RL A
		cpu.Rl_r(&cpu.R.A)
	case 0x18: //RR B
		cpu.Rr_r(&cpu.R.B)
	case 0x19: //RR C
		cpu.Rr_r(&cpu.R.C)
	case 0x1A: //RR D
		cpu.Rr_r(&cpu.R.D)
	case 0x1B: //RR E
		cpu.Rr_r(&cpu.R.E)
	case 0x1C: //RR H
		cpu.Rr_r(&cpu.R.H)
	case 0x1D: //RR L
		cpu.Rr_r(&cpu.R.L)
	case 0x1E: //RR (HL)
		cpu.Rr_hl()
	case 0x1F: //RR A
		cpu.Rr_r(&cpu.R.A)
	case 0x20: //SLA B
		cpu.Sla_r(&cpu.R.B)
	case 0x21: //SLA C
		cpu.Sla_r(&cpu.R.C)
	case 0x22: //SLA D
		cpu.Sla_r(&cpu.R.D)
	case 0x23: //SLA E
		cpu.Sla_r(&cpu.R.E)
	case 0x24: //SLA H
		cpu.Sla_r(&cpu.R.H)
	case 0x25: //SLA L
		cpu.Sla_r(&cpu.R.L)
	case 0x26: //SLA (HL)
		cpu.Sla_hl()
	case 0x27: //SLA A
		cpu.Sla_r(&cpu.R.A)
	case 0x28: //SRA B
		cpu.Sra_r(&cpu.R.B)
	case 0x29: //SRA C
		cpu.Sra_r(&cpu.R.C)
	case 0x2A: //SRA D
		cpu.Sra_r(&cpu.R.D)
	case 0x2B: //SRA E
		cpu.Sra_r(&cpu.R.E)
	case 0x2C: //SRA H
		cpu.Sra_r(&cpu.R.H)
	case 0x2D: //SRA L
		cpu.Sra_r(&cpu.R.L)
	case 0x2E: //SRA (HL)
		cpu.Sra_hl()
	case 0x2F: //SRA A
		cpu.Sra_r(&cpu.R.A)
	case 0x30: //SWAP B
		cpu.Swap_r(&cpu.R.B)
	case 0x31: //SWAP C
		cpu.Swap_r(&cpu.R.C)
	case 0x32: //SWAP D
		cpu.Swap_r(&cpu.R.D)
	case 0x33: //SWAP E
		cpu.Swap_r(&cpu.R.E)
	case 0x34: //SWAP H
		cpu.Swap_r(&cpu.R.H)
	case 0x35: //SWAP L
		cpu.Swap_r(&cpu.R.L)
	case 0x36: //SWAP (HL)
		cpu.Swap_hl()
	case 0x37: //SWAP A
		cpu.Swap_r(&cpu.R.A)
	case 0x38: //SRL B
		cpu.Srl_r(&cpu.R.B)
	case 0x39: //SRL C
		cpu.Srl_r(&cpu.R.C)
	case 0x3A: //SRL D
		cpu.Srl_r(&cpu.R.D)
	case 0x3B: //SRL E
		cpu.Srl_r(&cpu.R.E)
	case 0x3C: //SRL H
		cpu.Srl_r(&cpu.R.H)
	case 0x3D: //SRL L
		cpu.Srl_r(&cpu.R.L)
	case 0x3E: //SRL (HL)
		cpu.Srl_hl()
	case 0x3F: //SRL A
		cpu.Srl_r(&cpu.R.A)
	case 0x40: //BIT 0, B
		cpu.Bitb_r(0x00, &cpu.R.B)
	case 0x41: //BIT 0, C
		cpu.Bitb_r(0x00, &cpu.R.C)
	case 0x42: //BIT 0, D
		cpu.Bitb_r(0x00, &cpu.R.D)
	case 0x43: //BIT 0, E
		cpu.Bitb_r(0x00, &cpu.R.E)
	case 0x44: //BIT 0, H
		cpu.Bitb_r(0x00, &cpu.R.H)
	case 0x45: //BIT 0, L
		cpu.Bitb_r(0x00, &cpu.R.L)
	case 0x46: //BIT 0, (HL)
		cpu.Bitb_hl(0x00)
	case 0x47: //BIT 0, A
		cpu.Bitb_r(0x00, &cpu.R.A)
	case 0x48: //BIT 1, B
		cpu.Bitb_r(0x01, &cpu.R.B)
	case 0x49: //BIT 1, C
		cpu.Bitb_r(0x01, &cpu.R.C)
	case 0x4A: //BIT 1, D
		cpu.Bitb_r(0x01, &cpu.R.D)
	case 0x4B: //BIT 1, E
		cpu.Bitb_r(0x01, &cpu.R.E)
	case 0x4C: //BIT 1, H
		cpu.Bitb_r(0x01, &cpu.R.H)
	case 0x4D: //BIT 1, L
		cpu.Bitb_r(0x01, &cpu.R.L)
	case 0x4E: //BIT 1, (HL)
		cpu.Bitb_hl(0x01)
	case 0x4F: //BIT 1, A
		cpu.Bitb_r(0x01, &cpu.R.A)
	case 0x50: //BIT 2, B
		cpu.Bitb_r(0x02, &cpu.R.B)
	case 0x51: //BIT 2, C
		cpu.Bitb_r(0x02, &cpu.R.C)
	case 0x52: //BIT 2, D
		cpu.Bitb_r(0x02, &cpu.R.D)
	case 0x53: //BIT 2, E
		cpu.Bitb_r(0x02, &cpu.R.E)
	case 0x54: //BIT 2, H
		cpu.Bitb_r(0x02, &cpu.R.H)
	case 0x55: //BIT 2, L
		cpu.Bitb_r(0x02, &cpu.R.L)
	case 0x56: //BIT 2, (HL)
		cpu.Bitb_hl(0x02)
	case 0x57: //BIT 2, A
		cpu.Bitb_r(0x02, &cpu.R.A)
	case 0x58: //BIT 3, B
		cpu.Bitb_r(0x03, &cpu.R.B)
	case 0x59: //BIT 3, C
		cpu.Bitb_r(0x03, &cpu.R.C)
	case 0x5A: //BIT 3, D
		cpu.Bitb_r(0x03, &cpu.R.D)
	case 0x5B: //BIT 3, E
		cpu.Bitb_r(0x03, &cpu.R.E)
	case 0x5C: //BIT 3, H
		cpu.Bitb_r(0x03, &cpu.R.H)
	case 0x5D: //BIT 3, L
		cpu.Bitb_r(0x03, &cpu.R.L)
	case 0x5E: //BIT 3, (HL)
		cpu.Bitb_hl(0x03)
	case 0x5F: //BIT 3, A
		cpu.Bitb_r(0x03, &cpu.R.A)
	case 0x60: //BIT 4, B
		cpu.Bitb_r(0x04, &cpu.R.B)
	case 0x61: //BIT 4, C
		cpu.Bitb_r(0x04, &cpu.R.C)
	case 0x62: //BIT 4, D
		cpu.Bitb_r(0x04, &cpu.R.D)
	case 0x63: //BIT 4, E
		cpu.Bitb_r(0x04, &cpu.R.E)
	case 0x64: //BIT 4, H
		cpu.Bitb_r(0x04, &cpu.R.H)
	case 0x65: //BIT 4, L
		cpu.Bitb_r(0x04, &cpu.R.L)
	case 0x66: //BIT 4, (HL)
		cpu.Bitb_hl(0x04)
	case 0x67: //BIT 4, A
		cpu.Bitb_r(0x04, &cpu.R.A)
	case 0x68: //BIT 5, B
		cpu.Bitb_r(0x05, &cpu.R.B)
	case 0x69: //BIT 5, C
		cpu.Bitb_r(0x05, &cpu.R.C)
	case 0x6A: //BIT 5, D
		cpu.Bitb_r(0x05, &cpu.R.D)
	case 0x6B: //BIT 5, E
		cpu.Bitb_r(0x05, &cpu.R.E)
	case 0x6C: //BIT 5, H
		cpu.Bitb_r(0x05, &cpu.R.H)
	case 0x6D: //BIT 5, L
		cpu.Bitb_r(0x05, &cpu.R.L)
	case 0x6E: //BIT 5, (HL)
		cpu.Bitb_hl(0x05)
	case 0x6F: //BIT 5, A
		cpu.Bitb_r(0x05, &cpu.R.A)
	case 0x70: //BIT 6, B
		cpu.Bitb_r(0x06, &cpu.R.B)
	case 0x71: //BIT 6, C
		cpu.Bitb_r(0x06, &cpu.R.C)
	case 0x72: //BIT 6, D
		cpu.Bitb_r(0x06, &cpu.R.D)
	case 0x73: //BIT 6, E
		cpu.Bitb_r(0x06, &cpu.R.E)
	case 0x74: //BIT 6, H
		cpu.Bitb_r(0x06, &cpu.R.H)
	case 0x75: //BIT 6, L
		cpu.Bitb_r(0x06, &cpu.R.L)
	case 0x76: //BIT 6, (HL)
		cpu.Bitb_hl(0x06)
	case 0x77: //BIT 6, A
		cpu.Bitb_r(0x06, &cpu.R.A)
	case 0x78: //BIT 7, B
		cpu.Bitb_r(0x07, &cpu.R.B)
	case 0x79: //BIT 7, C
		cpu.Bitb_r(0x07, &cpu.R.C)
	case 0x7A: //BIT 7, D
		cpu.Bitb_r(0x07, &cpu.R.D)
	case 0x7B: //BIT 7, E
		cpu.Bitb_r(0x07, &cpu.R.E)
	case 0x7C: //BIT 7, H
		cpu.Bitb_r(0x07, &cpu.R.H)
	case 0x7D: //BIT 7, L
		cpu.Bitb_r(0x07, &cpu.R.L)
	case 0x7E: //BIT 7, (HL)
		cpu.Bitb_hl(0x07)
	case 0x7F: //BIT 7, A
		cpu.Bitb_r(0x07, &cpu.R.A)
	case 0x80: //RES 0, B
		cpu.Resb_r(0x00, &cpu.R.B)
	case 0x81: //RES 0, C
		cpu.Resb_r(0x00, &cpu.R.C)
	case 0x82: //RES 0, D
		cpu.Resb_r(0x00, &cpu.R.D)
	case 0x83: //RES 0, E
		cpu.Resb_r(0x00, &cpu.R.E)
	case 0x84: //RES 0, H
		cpu.Resb_r(0x00, &cpu.R.H)
	case 0x85: //RES 0, L
		cpu.Resb_r(0x00, &cpu.R.L)
	case 0x86: //RES 0,(HL) 
		cpu.Resb_hl(0x00)
	case 0x87: //RES 0, A
		cpu.Resb_r(0x00, &cpu.R.A)
	case 0x88: //RES 1, B
		cpu.Resb_r(0x01, &cpu.R.B)
	case 0x89: //RES 1, C
		cpu.Resb_r(0x01, &cpu.R.C)
	case 0x8A: //RES 1, D
		cpu.Resb_r(0x01, &cpu.R.D)
	case 0x8B: //RES 1, E
		cpu.Resb_r(0x01, &cpu.R.E)
	case 0x8C: //RES 1, H
		cpu.Resb_r(0x01, &cpu.R.H)
	case 0x8D: //RES 1, L
		cpu.Resb_r(0x01, &cpu.R.L)
	case 0x8E: //RES 1,(HL) 
		cpu.Resb_hl(0x01)
	case 0x8F: //RES 1, A
		cpu.Resb_r(0x01, &cpu.R.A)
	case 0x90: //RES 2, B
		cpu.Resb_r(0x02, &cpu.R.B)
	case 0x91: //RES 2, C
		cpu.Resb_r(0x02, &cpu.R.C)
	case 0x92: //RES 2, D
		cpu.Resb_r(0x02, &cpu.R.D)
	case 0x93: //RES 2, E
		cpu.Resb_r(0x02, &cpu.R.E)
	case 0x94: //RES 2, H
		cpu.Resb_r(0x02, &cpu.R.H)
	case 0x95: //RES 2, L
		cpu.Resb_r(0x02, &cpu.R.L)
	case 0x96: //RES 2,(HL) 
		cpu.Resb_hl(0x02)
	case 0x97: //RES 2, A
		cpu.Resb_r(0x02, &cpu.R.A)
	case 0x98: //RES 3, B
		cpu.Resb_r(0x03, &cpu.R.B)
	case 0x99: //RES 3, C
		cpu.Resb_r(0x03, &cpu.R.C)
	case 0x9A: //RES 3, D
		cpu.Resb_r(0x03, &cpu.R.D)
	case 0x9B: //RES 3, E
		cpu.Resb_r(0x03, &cpu.R.E)
	case 0x9C: //RES 3, H
		cpu.Resb_r(0x03, &cpu.R.H)
	case 0x9D: //RES 3, L
		cpu.Resb_r(0x03, &cpu.R.L)
	case 0x9E: //RES 3,(HL) 
		cpu.Resb_hl(0x03)
	case 0x9F: //RES 3, A
		cpu.Resb_r(0x03, &cpu.R.A)
	case 0xA0: //RES 4, B
		cpu.Resb_r(0x04, &cpu.R.B)
	case 0xA1: //RES 4, C
		cpu.Resb_r(0x04, &cpu.R.C)
	case 0xA2: //RES 4, D
		cpu.Resb_r(0x04, &cpu.R.D)
	case 0xA3: //RES 4, E
		cpu.Resb_r(0x04, &cpu.R.E)
	case 0xA4: //RES 4, H
		cpu.Resb_r(0x04, &cpu.R.H)
	case 0xA5: //RES 4, L
		cpu.Resb_r(0x04, &cpu.R.L)
	case 0xA6: //RES 4,(HL) 
		cpu.Resb_hl(0x04)
	case 0xA7: //RES 4, A
		cpu.Resb_r(0x04, &cpu.R.A)
	case 0xA8: //RES 5, B
		cpu.Resb_r(0x05, &cpu.R.B)
	case 0xA9: //RES 5, C
		cpu.Resb_r(0x05, &cpu.R.C)
	case 0xAA: //RES 5, D
		cpu.Resb_r(0x05, &cpu.R.D)
	case 0xAB: //RES 5, E
		cpu.Resb_r(0x05, &cpu.R.E)
	case 0xAC: //RES 5, H
		cpu.Resb_r(0x05, &cpu.R.H)
	case 0xAD: //RES 5, L
		cpu.Resb_r(0x05, &cpu.R.L)
	case 0xAE: //RES 5,(HL) 
		cpu.Resb_hl(0x05)
	case 0xAF: //RES 5, A
		cpu.Resb_r(0x05, &cpu.R.A)
	case 0xB0: //RES 6, B
		cpu.Resb_r(0x06, &cpu.R.B)
	case 0xB1: //RES 6, C
		cpu.Resb_r(0x06, &cpu.R.C)
	case 0xB2: //RES 6, D
		cpu.Resb_r(0x06, &cpu.R.D)
	case 0xB3: //RES 6, E
		cpu.Resb_r(0x06, &cpu.R.E)
	case 0xB4: //RES 6, H
		cpu.Resb_r(0x06, &cpu.R.H)
	case 0xB5: //RES 6, L
		cpu.Resb_r(0x06, &cpu.R.L)
	case 0xB6: //RES 6,(HL) 
		cpu.Resb_hl(0x06)
	case 0xB7: //RES 6, A
		cpu.Resb_r(0x06, &cpu.R.A)
	case 0xB8: //RES 7, B
		cpu.Resb_r(0x07, &cpu.R.B)
	case 0xB9: //RES 7, C
		cpu.Resb_r(0x07, &cpu.R.C)
	case 0xBA: //RES 7, D
		cpu.Resb_r(0x07, &cpu.R.D)
	case 0xBB: //RES 7, E
		cpu.Resb_r(0x07, &cpu.R.E)
	case 0xBC: //RES 7, H
		cpu.Resb_r(0x07, &cpu.R.H)
	case 0xBD: //RES 7, L
		cpu.Resb_r(0x07, &cpu.R.L)
	case 0xBE: //RES 7,(HL) 
		cpu.Resb_hl(0x07)
	case 0xBF: //RES 7, A
		cpu.Resb_r(0x07, &cpu.R.A)
	case 0xC0: //SET 0, B
		cpu.Setb_r(0x00, &cpu.R.B)
	case 0xC1: //SET 0, C
		cpu.Setb_r(0x00, &cpu.R.C)
	case 0xC2: //SET 0, D
		cpu.Setb_r(0x00, &cpu.R.D)
	case 0xC3: //SET 0, E
		cpu.Setb_r(0x00, &cpu.R.E)
	case 0xC4: //SET 0, H
		cpu.Setb_r(0x00, &cpu.R.H)
	case 0xC5: //SET 0, L
		cpu.Setb_r(0x00, &cpu.R.L)
	case 0xC6: //SET 0, (HL)
		cpu.Setb_hl(0x00)
	case 0xC7: //SET 0, A
		cpu.Setb_r(0x00, &cpu.R.A)
	case 0xC8: //SET 1, B
		cpu.Setb_r(0x01, &cpu.R.B)
	case 0xC9: //SET 1, C
		cpu.Setb_r(0x01, &cpu.R.C)
	case 0xCA: //SET 1, D
		cpu.Setb_r(0x01, &cpu.R.D)
	case 0xCB: //SET 1, E
		cpu.Setb_r(0x01, &cpu.R.E)
	case 0xCC: //SET 1, H
		cpu.Setb_r(0x01, &cpu.R.H)
	case 0xCD: //SET 1, L
		cpu.Setb_r(0x01, &cpu.R.L)
	case 0xCE: //SET 1, (HL)
		cpu.Setb_hl(0x01)
	case 0xCF: //SET 1, A
		cpu.Setb_r(0x01, &cpu.R.A)
	case 0xD0: //SET 2, B
		cpu.Setb_r(0x02, &cpu.R.B)
	case 0xD1: //SET 2, C
		cpu.Setb_r(0x02, &cpu.R.C)
	case 0xD2: //SET 2, D
		cpu.Setb_r(0x02, &cpu.R.D)
	case 0xD3: //SET 2, E
		cpu.Setb_r(0x02, &cpu.R.E)
	case 0xD4: //SET 2, H
		cpu.Setb_r(0x02, &cpu.R.H)
	case 0xD5: //SET 2, L
		cpu.Setb_r(0x02, &cpu.R.L)
	case 0xD6: //SET 2, (HL)
		cpu.Setb_hl(0x02)
	case 0xD7: //SET 2, A
		cpu.Setb_r(0x02, &cpu.R.A)
	case 0xD8: //SET 3, B
		cpu.Setb_r(0x03, &cpu.R.B)
	case 0xD9: //SET 3, C
		cpu.Setb_r(0x03, &cpu.R.C)
	case 0xDA: //SET 3, D
		cpu.Setb_r(0x03, &cpu.R.D)
	case 0xDB: //SET 3, E
		cpu.Setb_r(0x03, &cpu.R.E)
	case 0xDC: //SET 3, H
		cpu.Setb_r(0x03, &cpu.R.H)
	case 0xDD: //SET 3, L
		cpu.Setb_r(0x03, &cpu.R.L)
	case 0xDE: //SET 3, (HL)
		cpu.Setb_hl(0x03)
	case 0xDF: //SET 3, A
		cpu.Setb_r(0x03, &cpu.R.A)
	case 0xE0: //SET 4, B
		cpu.Setb_r(0x04, &cpu.R.B)
	case 0xE1: //SET 4, C
		cpu.Setb_r(0x04, &cpu.R.C)
	case 0xE2: //SET 4, D
		cpu.Setb_r(0x04, &cpu.R.D)
	case 0xE3: //SET 4, E
		cpu.Setb_r(0x04, &cpu.R.E)
	case 0xE4: //SET 4, H
		cpu.Setb_r(0x04, &cpu.R.H)
	case 0xE5: //SET 4, L
		cpu.Setb_r(0x04, &cpu.R.L)
	case 0xE6: //SET 4, (HL)
		cpu.Setb_hl(0x04)
	case 0xE7: //SET 4, A
		cpu.Setb_r(0x04, &cpu.R.A)
	case 0xE8: //SET 5, B
		cpu.Setb_r(0x05, &cpu.R.B)
	case 0xE9: //SET 5, C
		cpu.Setb_r(0x05, &cpu.R.C)
	case 0xEA: //SET 5, D
		cpu.Setb_r(0x05, &cpu.R.D)
	case 0xEB: //SET 5, E
		cpu.Setb_r(0x05, &cpu.R.E)
	case 0xEC: //SET 5, H
		cpu.Setb_r(0x05, &cpu.R.H)
	case 0xED: //SET 5, L
		cpu.Setb_r(0x05, &cpu.R.L)
	case 0xEE: //SET 5, (HL)
		cpu.Setb_hl(0x05)
	case 0xEF: //SET 5, A
		cpu.Setb_r(0x05, &cpu.R.A)
	case 0xF0: //SET 6, B
		cpu.Setb_r(0x06, &cpu.R.B)
	case 0xF1: //SET 6, C
		cpu.Setb_r(0x06, &cpu.R.C)
	case 0xF2: //SET 6, D
		cpu.Setb_r(0x06, &cpu.R.D)
	case 0xF3: //SET 6, E
		cpu.Setb_r(0x06, &cpu.R.E)
	case 0xF4: //SET 6, H
		cpu.Setb_r(0x06, &cpu.R.H)
	case 0xF5: //SET 6, L
		cpu.Setb_r(0x06, &cpu.R.L)
	case 0xF6: //SET 6, (HL)
		cpu.Setb_hl(0x06)
	case 0xF7: //SET 6, A
		cpu.Setb_r(0x06, &cpu.R.A)
	case 0xF8: //SET 7, B
		cpu.Setb_r(0x07, &cpu.R.B)
	case 0xF9: //SET 7, C
		cpu.Setb_r(0x07, &cpu.R.C)
	case 0xFA: //SET 7, D
		cpu.Setb_r(0x07, &cpu.R.D)
	case 0xFB: //SET 7, E
		cpu.Setb_r(0x07, &cpu.R.E)
	case 0xFC: //SET 7, H
		cpu.Setb_r(0x07, &cpu.R.H)
	case 0xFD: //SET 7, L
		cpu.Setb_r(0x07, &cpu.R.L)
	case 0xFE: //SET 7, (HL)
		cpu.Setb_hl(0x07)
	case 0xFF: //SET 7, A
		cpu.Setb_r(0x07, &cpu.R.A)

	default:
		log.Fatalf(PREFIX+" Invalid/Unknown instruction %X", Opcode)
	}
}

func (cpu *Z80) Dispatch(Opcode byte) {
	switch Opcode {
	case 0x00: //NOP
		cpu.NOP()
	case 0x01: //LD BC, nn
		cpu.LDn_nn(&cpu.R.B, &cpu.R.C)
	case 0x02: //LD (BC), A
		cpu.LDbc_r(&cpu.R.A)
	case 0x03: //INC BC
		cpu.Inc_rr(&cpu.R.B, &cpu.R.C)
	case 0x04: //INC B
		cpu.Inc_r(&cpu.R.B)
	case 0x05: //DEC B
		cpu.Dec_r(&cpu.R.B)
	case 0x06: //LD B,n
		cpu.LDrn(&cpu.R.B)
	case 0x07: //RLCA
		cpu.RLCA()
	case 0x08: //LD nn, SP
		cpu.LDnn_SP()
	case 0x09: //ADD HL,BC
		cpu.Addhl_rr(&cpu.R.B, &cpu.R.C)
	case 0x0A: //LD A, (BC)
		cpu.LDr_bc(&cpu.R.A)
	case 0x0B: //DEC BC
		cpu.Dec_rr(&cpu.R.B, &cpu.R.C)
	case 0x0C: //INC C
		cpu.Inc_r(&cpu.R.C)
	case 0x0D: //DEC C
		cpu.Dec_r(&cpu.R.C)
	case 0x0E: //LD C,n
		cpu.LDrn(&cpu.R.C)
	case 0x0F: //RRCA
		cpu.RRCA()
	case 0x10: //STOP
		cpu.Stop()
	case 0x11: //LD DE, nn
		cpu.LDn_nn(&cpu.R.D, &cpu.R.E)
	case 0x12: //LD (DE), A
		cpu.LDde_r(&cpu.R.A)
	case 0x13: //INC DE
		cpu.Inc_rr(&cpu.R.D, &cpu.R.E)
	case 0x14: //INC D
		cpu.Inc_r(&cpu.R.D)
	case 0x15: //DEC D
		cpu.Dec_r(&cpu.R.D)
	case 0x16: //LD D,n
		cpu.LDrn(&cpu.R.D)
	case 0x17: //RLA
		cpu.RLA()
	case 0x18: //JR n
		cpu.JR_n()
	case 0x19: //ADD HL,DE
		cpu.Addhl_rr(&cpu.R.D, &cpu.R.E)
	case 0x1A: //LD A, (DE)
		cpu.LDr_de(&cpu.R.A)
	case 0x1B: //DEC DE
		cpu.Dec_rr(&cpu.R.D, &cpu.R.E)
	case 0x1C: //INC E
		cpu.Inc_r(&cpu.R.E)
	case 0x1D: //DEC E
		cpu.Dec_r(&cpu.R.E)
	case 0x1E: //LD E,n
		cpu.LDrn(&cpu.R.E)
	case 0x1F: //RRA
		cpu.RRA()
	case 0x20: //JR NZ,n
		cpu.JRcc_nn(Z, false)
	case 0x21: //LD HL, nn
		cpu.LDn_nn(&cpu.R.H, &cpu.R.L)
	case 0x22: //LDI (HL), A
		cpu.LDIhl_r(&cpu.R.A)
	case 0x23: //INC HL
		cpu.Inc_rr(&cpu.R.H, &cpu.R.L)
	case 0x24: //INC H
		cpu.Inc_r(&cpu.R.H)
	case 0x25: //DEC H
		cpu.Dec_r(&cpu.R.H)
	case 0x26: //LD H,n
		cpu.LDrn(&cpu.R.H)
	case 0x27: //DAA
		cpu.Daa()
	case 0x28: //JR Z,n
		cpu.JRcc_nn(Z, true)
	case 0x29: //ADD HL,HL
		cpu.Addhl_rr(&cpu.R.H, &cpu.R.L)
	case 0x2A: //LDI A, (HL)
		cpu.LDIr_hl(&cpu.R.A)
	case 0x2B: //DEC HL
		cpu.Dec_rr(&cpu.R.H, &cpu.R.L)
	case 0x2C: //INC L
		cpu.Inc_r(&cpu.R.L)
	case 0x2D: //DEC L
		cpu.Dec_r(&cpu.R.L)
	case 0x2E: //LD L,n
		cpu.LDrn(&cpu.R.L)
	case 0x2F: //CPL
		cpu.CPL()
	case 0x30: //JR NZ,n
		cpu.JRcc_nn(C, false)
	case 0x31: //LD SP, nn
		cpu.LDSP_nn()
	case 0x32: //LDD (HL), A
		cpu.LDDhl_r(&cpu.R.A)
	case 0x33: //INC SP
		cpu.Inc_sp()
	case 0x34: //INC (HL)
		cpu.Inc_hl()
	case 0x35: //DEC (HL)
		cpu.Dec_hl()
	case 0x36: //LD (HL), n
		cpu.LDhl_n()
	case 0x37: //SCF
		cpu.SCF()
	case 0x38: //JR C,n
		cpu.JRcc_nn(C, true)
	case 0x39: //ADD HL,SP
		cpu.Addhl_sp()
	case 0x3A: //LDD A, (HL)
		cpu.LDDr_hl(&cpu.R.A)
	case 0x3B: //DEC SP
		cpu.Dec_sp()
	case 0x3C: //INC A
		cpu.Inc_r(&cpu.R.A)
	case 0x3D: //DEC A
		cpu.Dec_r(&cpu.R.A)
	case 0x3E: //LD A, n
		cpu.LDrn(&cpu.R.A)
	case 0x3F: //CCF
		cpu.CCF()
	case 0x40: //LD B, B
		cpu.LDrr(&cpu.R.B, &cpu.R.B)
	case 0x41: //LD B, C
		cpu.LDrr(&cpu.R.B, &cpu.R.C)
	case 0x42: //LD B, D
		cpu.LDrr(&cpu.R.B, &cpu.R.D)
	case 0x43: //LD B, E
		cpu.LDrr(&cpu.R.B, &cpu.R.E)
	case 0x44: //LD B, H
		cpu.LDrr(&cpu.R.B, &cpu.R.H)
	case 0x45: //LD B, L
		cpu.LDrr(&cpu.R.B, &cpu.R.L)
	case 0x46: //LD B, (HL)
		cpu.LDr_hl(&cpu.R.B)
	case 0x47: //LD B, A
		cpu.LDrr(&cpu.R.B, &cpu.R.A)
	case 0x48: //LD C, B
		cpu.LDrr(&cpu.R.C, &cpu.R.B)
	case 0x49: //LD C, C
		cpu.LDrr(&cpu.R.C, &cpu.R.C)
	case 0x4A: //LD C, D
		cpu.LDrr(&cpu.R.C, &cpu.R.D)
	case 0x4B: //LD C, E
		cpu.LDrr(&cpu.R.C, &cpu.R.E)
	case 0x4C: //LD C, H
		cpu.LDrr(&cpu.R.C, &cpu.R.H)
	case 0x4D: //LD C, L
		cpu.LDrr(&cpu.R.C, &cpu.R.L)
	case 0x4E: //LD C, (HL)
		cpu.LDr_hl(&cpu.R.C)
	case 0x4F: //LD C, A
		cpu.LDrr(&cpu.R.C, &cpu.R.A)
	case 0x50: //LD D, B
		cpu.LDrr(&cpu.R.D, &cpu.R.B)
	case 0x51: //LD D, C
		cpu.LDrr(&cpu.R.D, &cpu.R.C)
	case 0x52: //LD D, D
		cpu.LDrr(&cpu.R.D, &cpu.R.D)
	case 0x53: //LD D, E
		cpu.LDrr(&cpu.R.D, &cpu.R.E)
	case 0x54: //LD D, H
		cpu.LDrr(&cpu.R.D, &cpu.R.H)
	case 0x55: //LD D, L
		cpu.LDrr(&cpu.R.D, &cpu.R.L)
	case 0x56: //LD D, (HL)
		cpu.LDr_hl(&cpu.R.D)
	case 0x57: //LD D, A
		cpu.LDrr(&cpu.R.D, &cpu.R.A)
	case 0x58: //LD E, B
		cpu.LDrr(&cpu.R.E, &cpu.R.B)
	case 0x59: //LD E, C
		cpu.LDrr(&cpu.R.E, &cpu.R.C)
	case 0x5A: //LD E, D
		cpu.LDrr(&cpu.R.E, &cpu.R.D)
	case 0x5B: //LD E, E
		cpu.LDrr(&cpu.R.E, &cpu.R.E)
	case 0x5C: //LD E, H
		cpu.LDrr(&cpu.R.E, &cpu.R.H)
	case 0x5D: //LD E, L
		cpu.LDrr(&cpu.R.E, &cpu.R.L)
	case 0x5E: //LD E, (HL)
		cpu.LDr_hl(&cpu.R.E)
	case 0x5F: //LD E, A
		cpu.LDrr(&cpu.R.E, &cpu.R.A)
	case 0x60: //LD H, B
		cpu.LDrr(&cpu.R.H, &cpu.R.B)
	case 0x61: //LD H, C
		cpu.LDrr(&cpu.R.H, &cpu.R.C)
	case 0x62: //LD H, D
		cpu.LDrr(&cpu.R.H, &cpu.R.D)
	case 0x63: //LD H, E
		cpu.LDrr(&cpu.R.H, &cpu.R.E)
	case 0x64: //LD H, H
		cpu.LDrr(&cpu.R.H, &cpu.R.H)
	case 0x65: //LD H, L
		cpu.LDrr(&cpu.R.H, &cpu.R.L)
	case 0x66: //LD H, (HL)
		cpu.LDr_hl(&cpu.R.H)
	case 0x67: //LD H, A
		cpu.LDrr(&cpu.R.H, &cpu.R.A)
	case 0x68: //LD L, B
		cpu.LDrr(&cpu.R.L, &cpu.R.B)
	case 0x69: //LD L, C
		cpu.LDrr(&cpu.R.L, &cpu.R.C)
	case 0x6A: //LD L, D
		cpu.LDrr(&cpu.R.L, &cpu.R.D)
	case 0x6B: //LD L, E
		cpu.LDrr(&cpu.R.L, &cpu.R.E)
	case 0x6C: //LD L, H
		cpu.LDrr(&cpu.R.L, &cpu.R.H)
	case 0x6D: //LD L, L
		cpu.LDrr(&cpu.R.L, &cpu.R.L)
	case 0x6E: //LD L, (HL)
		cpu.LDr_hl(&cpu.R.L)
	case 0x6F: //LD L, A
		cpu.LDrr(&cpu.R.L, &cpu.R.A)
	case 0x70: //LD (HL), B
		cpu.LDhl_r(&cpu.R.B)
	case 0x71: //LD (HL), C
		cpu.LDhl_r(&cpu.R.C)
	case 0x72: //LD (HL), D
		cpu.LDhl_r(&cpu.R.D)
	case 0x73: //LD (HL), E
		cpu.LDhl_r(&cpu.R.E)
	case 0x74: //LD (HL), H
		cpu.LDhl_r(&cpu.R.H)
	case 0x75: //LD (HL), L
		cpu.LDhl_r(&cpu.R.L)
	case 0x76: //HALT
		cpu.HALT()
	case 0x77: //LD (HL), A
		cpu.LDhl_r(&cpu.R.A)
	case 0x78: //LD A, B
		cpu.LDrr(&cpu.R.A, &cpu.R.B)
	case 0x79: //LD A, C
		cpu.LDrr(&cpu.R.A, &cpu.R.C)
	case 0x7A: //LD A, D
		cpu.LDrr(&cpu.R.A, &cpu.R.D)
	case 0x7B: //LD A, E
		cpu.LDrr(&cpu.R.A, &cpu.R.E)
	case 0x7C: //LD A, H
		cpu.LDrr(&cpu.R.A, &cpu.R.H)
	case 0x7D: //LD A, L
		cpu.LDrr(&cpu.R.A, &cpu.R.L)
	case 0x7E: //LD A, (HL)
		cpu.LDr_hl(&cpu.R.A)
	case 0x7F: //LD A, A
		cpu.LDrr(&cpu.R.A, &cpu.R.A)
	case 0x80: //ADD A, B
		cpu.AddA_r(&cpu.R.B)
	case 0x81: //ADD A, C
		cpu.AddA_r(&cpu.R.C)
	case 0x82: //ADD A, D
		cpu.AddA_r(&cpu.R.D)
	case 0x83: //ADD A, E
		cpu.AddA_r(&cpu.R.E)
	case 0x84: //ADD A, H
		cpu.AddA_r(&cpu.R.H)
	case 0x85: //ADD A, L
		cpu.AddA_r(&cpu.R.L)
	case 0x86: //ADD A,(HL)
		cpu.AddA_hl()
	case 0x87: //ADD A, A
		cpu.AddA_r(&cpu.R.A)
	case 0x88: //ADC A, B
		cpu.AddCA_r(&cpu.R.B)
	case 0x89: //ADC A, C
		cpu.AddCA_r(&cpu.R.C)
	case 0x8A: //ADC A, D
		cpu.AddCA_r(&cpu.R.D)
	case 0x8B: //ADC A, E
		cpu.AddCA_r(&cpu.R.E)
	case 0x8C: //ADC A, H
		cpu.AddCA_r(&cpu.R.H)
	case 0x8D: //ADC A, L
		cpu.AddCA_r(&cpu.R.L)
	case 0x8E: //ADC A, (HL)
		cpu.AddCA_hl()
	case 0x8F: //ADC A, A
		cpu.AddCA_r(&cpu.R.A)
	case 0x90: // SUB A, B
		cpu.SubA_r(&cpu.R.B)
	case 0x91: // SUB A, C
		cpu.SubA_r(&cpu.R.C)
	case 0x92: // SUB A, D
		cpu.SubA_r(&cpu.R.D)
	case 0x93: // SUB A, E
		cpu.SubA_r(&cpu.R.E)
	case 0x94: // SUB A, H
		cpu.SubA_r(&cpu.R.H)
	case 0x95: // SUB A, L
		cpu.SubA_r(&cpu.R.L)
	case 0x96: //SUB A, (HL)
		cpu.SubA_hl()
	case 0x97: // SUB A, A
		cpu.SubA_r(&cpu.R.A)
	case 0x98: // SBC A, B
		cpu.SubAC_r(&cpu.R.B)
	case 0x99: // SBC A, C
		cpu.SubAC_r(&cpu.R.C)
	case 0x9A: // SBC A, D
		cpu.SubAC_r(&cpu.R.D)
	case 0x9B: // SBC A, E
		cpu.SubAC_r(&cpu.R.E)
	case 0x9C: // SBC A, H
		cpu.SubAC_r(&cpu.R.H)
	case 0x9D: // SBC A, L
		cpu.SubAC_r(&cpu.R.L)
	case 0x9E: //SBC A, (HL)
		cpu.SubAC_hl()
	case 0x9F: // SBC A, A
		cpu.SubAC_r(&cpu.R.A)
	case 0xA0: //AND A, B
		cpu.AndA_r(&cpu.R.B)
	case 0xA1: //AND A, C
		cpu.AndA_r(&cpu.R.C)
	case 0xA2: //AND A, D
		cpu.AndA_r(&cpu.R.D)
	case 0xA3: //AND A, E
		cpu.AndA_r(&cpu.R.E)
	case 0xA4: //AND A, H
		cpu.AndA_r(&cpu.R.H)
	case 0xA5: //AND A, L
		cpu.AndA_r(&cpu.R.L)
	case 0xA6: //AND A, (HL)
		cpu.AndA_hl()
	case 0xA7: //AND A, A
		cpu.AndA_r(&cpu.R.A)
	case 0xA8: //XOR A, B
		cpu.XorA_r(&cpu.R.B)
	case 0xA9: //XOR A, C
		cpu.XorA_r(&cpu.R.C)
	case 0xAA: //XOR A, D
		cpu.XorA_r(&cpu.R.D)
	case 0xAB: //XOR A, E
		cpu.XorA_r(&cpu.R.E)
	case 0xAC: //XOR A, H
		cpu.XorA_r(&cpu.R.H)
	case 0xAD: //XOR A, L
		cpu.XorA_r(&cpu.R.L)
	case 0xAE: //XOR A,(HL)
		cpu.XorA_hl()
	case 0xAF: //XOR A, A
		cpu.XorA_r(&cpu.R.A)
	case 0xB0: //OR A, B
		cpu.OrA_r(&cpu.R.B)
	case 0xB1: //OR A, C
		cpu.OrA_r(&cpu.R.C)
	case 0xB2: //OR A, D
		cpu.OrA_r(&cpu.R.D)
	case 0xB3: //OR A, E
		cpu.OrA_r(&cpu.R.E)
	case 0xB4: //OR A, H
		cpu.OrA_r(&cpu.R.H)
	case 0xB5: //OR A, L
		cpu.OrA_r(&cpu.R.L)
	case 0xB6: //OR A,(HL)
		cpu.OrA_hl()
	case 0xB7: //OR A, A
		cpu.OrA_r(&cpu.R.A)
	case 0xB8: //CP A, B
		cpu.CPA_r(&cpu.R.B)
	case 0xB9: //CP A, C
		cpu.CPA_r(&cpu.R.C)
	case 0xBA: //CP A, D
		cpu.CPA_r(&cpu.R.D)
	case 0xBB: //CP A, E
		cpu.CPA_r(&cpu.R.E)
	case 0xBC: //CP A, H
		cpu.CPA_r(&cpu.R.H)
	case 0xBD: //CP A, L
		cpu.CPA_r(&cpu.R.L)
	case 0xBE: //CP A, (HL)
		cpu.CPA_hl()
	case 0xBF: //CP A, A
		cpu.CPA_r(&cpu.R.A)
	case 0xC0: //RET NZ
		cpu.Retcc(Z, false)
	case 0xC1: //POP BC
		cpu.Pop_nn(&cpu.R.B, &cpu.R.C)
	case 0xC2: //JP NZ,nn
		cpu.JPcc_nn(Z, false)
	case 0xC3: //JP nn
		cpu.JP_nn()
	case 0xC4: //CALL NZ, nn
		cpu.Callcc_nn(Z, false)
	case 0xC5: //PUSH BC
		cpu.Push_nn(&cpu.R.B, &cpu.R.C)
	case 0xC6: //ADD A,#
		cpu.AddA_n()
	case 0xC7: //RST n
		cpu.Rst(0x00)
	case 0xC8: //RET Z
		cpu.Retcc(Z, true)
	case 0xC9: //RET
		cpu.Ret()
	case 0xCA: //JP Z,nn
		cpu.JPcc_nn(Z, true)
	case 0xCC: //CALL Z, nn
		cpu.Callcc_nn(Z, true)
	case 0xCD: //CALL nn
		cpu.Call_nn()
	case 0xCE: //ADC A, n
		cpu.AddCA_n()
	case 0xCF: //RST n
		cpu.Rst(0x08)
	case 0xD0: //RET NC
		cpu.Retcc(C, false)
	case 0xD1: //POP DE
		cpu.Pop_nn(&cpu.R.D, &cpu.R.E)
	case 0xD2: //JP NC,nn
		cpu.JPcc_nn(C, false)
	case 0xD4: //CALL NC, nn
		cpu.Callcc_nn(C, false)
	case 0xD5: //PUSH DE
		cpu.Push_nn(&cpu.R.D, &cpu.R.E)
	case 0xD6: //SUB A, n
		cpu.SubA_n()
	case 0xD7: //RST n
		cpu.Rst(0x10)
	case 0xD8: //RET C
		cpu.Retcc(C, true)
	case 0xD9: //RETI
		cpu.Ret_i()
	case 0xDA: //JP C,nn
		cpu.JPcc_nn(C, true)
	case 0xDC: //CALL C, nn
		cpu.Callcc_nn(C, true)
	case 0xDE: //SBC A, n
		cpu.SubAC_n()
	case 0xDF: //RST n
		cpu.Rst(0x18)
	case 0xE0: //LDH n, A
		cpu.LDHn_r(&cpu.R.A)
	case 0xE1: //POP HL
		cpu.Pop_nn(&cpu.R.H, &cpu.R.L)
	case 0xE2: //LD (C),A
		cpu.LDffplusc_r(&cpu.R.A)
	case 0xE5: //PUSH HL
		cpu.Push_nn(&cpu.R.H, &cpu.R.L)
	case 0xE6: //AND A, n
		cpu.AndA_n()
	case 0xE7: //RST n
		cpu.Rst(0x20)
	case 0xE8: //ADD SP,n
		cpu.Addsp_n()
	case 0xE9: //JP (HL)
		cpu.JP_hl()
	case 0xEA: //LD (nn), A
		cpu.LDnn_r(&cpu.R.A)
	case 0xEE: //XOR A, n
		cpu.XorA_n()
	case 0xEF: //RST n
		cpu.Rst(0x28)
	case 0xF0: //LDH r, n
		cpu.LDHr_n(&cpu.R.A)
	case 0xF1: //POP AF
		cpu.Pop_nn(&cpu.R.A, &cpu.R.F)
	case 0xF2: //LD A,(C)
		cpu.LDr_ffplusc(&cpu.R.A)
	case 0xF3: //DI
		cpu.DI()
	case 0xF5: //PUSH AF
		cpu.Push_nn(&cpu.R.A, &cpu.R.F)
	case 0xF6: //OR A, n
		cpu.OrA_n()
	case 0xF7: //RST n
		cpu.Rst(0x30)
	case 0xF8: //LDHL SP, n
		cpu.LDHLSP_n()
	case 0xF9: //LD SP, HL
		cpu.LDSP_hl()
	case 0xFA: //LD A, (nn)
		cpu.LDr_nn(&cpu.R.A)
	case 0xFB: //EI
		cpu.EI()
	case 0xFE: //CP A, n
		cpu.CPA_n()
	case 0xFF: //RST n
		cpu.Rst(0x38)
	default:
		log.Fatalf(PREFIX+" Invalid/Unknown instruction %X", Opcode)
	}
}

func (cpu *Z80) Step() int {
	if err := cpu.Validate(); err != nil {
		log.Fatalln(PREFIX, err)
	}

	var Opcode byte = cpu.ReadByte(cpu.PC)
	var ok bool = false

	if Opcode == 0xCB {
		cpu.IncrementPC(1)
		Opcode = cpu.ReadByte(cpu.PC)
		cpu.CurrentInstruction, ok = cpu.DecodeCB(Opcode)
		if !ok {
			panic(fmt.Sprintf("No instruction found for opcode: %X\n%s", Opcode, cpu.String()))
		}
		cpu.CurrentInstruction = cpu.Compile(cpu.CurrentInstruction)
		cpu.DispatchCB(Opcode)
	} else {
		cpu.CurrentInstruction, ok = cpu.Decode(Opcode)
		if !ok {
			panic(fmt.Sprintf("No instruction found for opcode: %X\n%s", Opcode, cpu.String()))
		}
		cpu.CurrentInstruction = cpu.Compile(cpu.CurrentInstruction)
		cpu.Dispatch(Opcode)
	}

	//this is put in place to check whether the PC has been altered by an instruction. If it has then don't
	//do any incrementing
	if cpu.PCJumped {
		cpu.PCJumped = false
	} else {
		cpu.IncrementPC(cpu.CurrentInstruction.OperandsSize + 1)
	}

	//calculate cycles
	cpu.LastInstrCycle.M += cpu.CurrentInstruction.Cycles
	cpu.MachineCycles.M += cpu.LastInstrCycle.M

	t := cpu.LastInstrCycle.T()
	cpu.LastInstrCycle.Reset()
	return t
}

func (cpu *Z80) pushByteToStack(b byte) {
	cpu.SP--
	cpu.WriteByte(cpu.SP, b)
}

func (cpu *Z80) pushWordToStack(word types.Word) {
	cpu.SP -= 2
	cpu.WriteWord(cpu.SP, word)
}

func (cpu *Z80) popByteFromStack() byte {
	var b byte = cpu.ReadByte(cpu.SP)
	cpu.SP++
	return b
}

func (cpu *Z80) popWordFromStack() types.Word {
	var w types.Word = cpu.ReadWord(cpu.SP)
	cpu.SP += 2
	return w
}

func (cpu *Z80) ReadByte(addr types.Word) byte {
	if err := cpu.Validate(); err != nil {
		log.Fatalln(PREFIX, err)
	}

	return cpu.mmu.ReadByte(addr)
}

func (cpu *Z80) ReadWord(addr types.Word) types.Word {
	if err := cpu.Validate(); err != nil {
		log.Fatalln(PREFIX, err)
	}

	return cpu.mmu.ReadWord(addr)
}

func (cpu *Z80) WriteByte(addr types.Word, value byte) {
	if err := cpu.Validate(); err != nil {
		log.Fatalln(PREFIX, err)
	}

	cpu.mmu.WriteByte(addr, value)
}

func (cpu *Z80) WriteWord(addr types.Word, value types.Word) {
	if err := cpu.Validate(); err != nil {
		log.Fatalln(PREFIX, err)
	}

	cpu.mmu.WriteWord(addr, value)
}

func (cpu *Z80) Compile(instruction Instruction) Instruction {
	switch instruction.OperandsSize {
	case 1:
		instruction.Operands[0] = cpu.mmu.ReadByte(cpu.PC + 1)
	case 2:
		instruction.Operands[0] = cpu.mmu.ReadByte(cpu.PC + 1)
		instruction.Operands[1] = cpu.mmu.ReadByte(cpu.PC + 2)
	}

	return instruction
}

func (cpu *Z80) Decode(instruction byte) (Instruction, bool) {
	ins, ok := Instructions[instruction]
	return ins, ok
}

func (cpu *Z80) DecodeCB(instruction byte) (Instruction, bool) {
	ins, ok := InstructionsCB[instruction]
	return ins, ok
}

// INSTRUCTIONS START
//-----------------------------------------------------------------------

//LD r,n
//Load value (n) from memory address in the PC into register (r) and increment PC by 1 
func (cpu *Z80) LDrn(r *byte) {
	*r = cpu.CurrentInstruction.Operands[0]
}

//LD r,r
//Load value from register (r2) into register (r1)
func (cpu *Z80) LDrr(r1 *byte, r2 *byte) {
	*r1 = *r2
}

//LD r,(HL)
//Load value from memory address located in register pair (HL) into register (r)
func (cpu *Z80) LDr_hl(r *byte) {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)

	*r = value
}

//LD (HL),r
//Load value from register (r) into memory address located at register pair (HL)
func (cpu *Z80) LDhl_r(r *byte) {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = *r

	cpu.WriteByte(HL, value)
}

//LD (BC),r
//Load value from register (r) into memory address located at register pair (BC)
func (cpu *Z80) LDbc_r(r *byte) {
	var BC types.Word = types.Word(utils.JoinBytes(cpu.R.B, cpu.R.C))
	var value byte = *r

	cpu.WriteByte(BC, value)
}

//LD (DE),r
//Load value from register (r) into memory address located at register pair (DE)
func (cpu *Z80) LDde_r(r *byte) {

	var DE types.Word = types.Word(utils.JoinBytes(cpu.R.D, cpu.R.E))
	var value byte = *r

	cpu.WriteByte(DE, value)
}

//LD nn,r
//Load value from register (r) and put it in memory address (nn) taken from the next 2 bytes of memory from the PC. Increment the PC by 2
func (cpu *Z80) LDnn_r(r *byte) {
	var ls byte = cpu.CurrentInstruction.Operands[0]
	var hs byte = cpu.CurrentInstruction.Operands[1]
	var resultAddr types.Word = types.Word(utils.JoinBytes(hs, ls))
	cpu.WriteByte(resultAddr, *r)
}

//LD (HL),n
//Load the value (n) from the memory address in the PC and put it in the memory address designated by register pair (HL)
func (cpu *Z80) LDhl_n() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.CurrentInstruction.Operands[0]
	cpu.WriteByte(HL, value)
}

//LD r, (BC)
//Load the value (n) located in memory address stored in register pair (BC) and put it in register (r)
func (cpu *Z80) LDr_bc(r *byte) {

	var BC types.Word = types.Word(utils.JoinBytes(cpu.R.B, cpu.R.C))
	var value byte = cpu.ReadByte(BC)

	*r = value

	//set clock values
}

//LD r, (DE)
//Load the value (n) located in memory address stored in register pair (DE) and put it in register (r)
func (cpu *Z80) LDr_de(r *byte) {

	var DE types.Word = types.Word(utils.JoinBytes(cpu.R.D, cpu.R.E))
	var value byte = cpu.ReadByte(DE)

	*r = value

	//set clock values
}

//LD r, nn
//Load the value in memory address defined from the next two bytes relative to the PC and store it in register (r). Increment the PC by 2
func (cpu *Z80) LDr_nn(r *byte) {
	var ls byte = cpu.CurrentInstruction.Operands[0]
	var hs byte = cpu.CurrentInstruction.Operands[1]

	var nn types.Word = types.Word(utils.JoinBytes(hs, ls))
	var value byte = cpu.ReadByte(nn)
	*r = value
}

//LD r,(C)
//Load the value from memory addressed 0xFF00 + value in register C. Store it in register (r)
func (cpu *Z80) LDr_ffplusc(r *byte) {
	var valueAddr types.Word = 0xFF00 + types.Word(cpu.R.C)
	*r = cpu.ReadByte(valueAddr)

	//set clock values
}

//LD (C),r
//Load the value from register (r) and store it in memory addressed 0xFF00 + value in register C. 
func (cpu *Z80) LDffplusc_r(r *byte) {
	var valueAddr types.Word = 0xFF00 + types.Word(cpu.R.C)
	cpu.WriteByte(valueAddr, *r)

	//set clock values
}

//LDD r, (HL)
//Load the value from memory addressed in register pair (HL) and store it in register R. Decrement the HL registers
func (cpu *Z80) LDDr_hl(r *byte) {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	*r = cpu.ReadByte(HL)

	//decrement HL registers
	cpu.R.L -= 1

	//decrement H too if L is 0xFF
	if cpu.R.L == 0xFF {
		cpu.R.H -= 1
	}
}

//LDD (HL), r
//Load the value in register (r) and store in memory addressed in register pair (HL). Decrement the HL registers
func (cpu *Z80) LDDhl_r(r *byte) {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	cpu.WriteByte(HL, *r)

	//decrement HL registers
	cpu.R.L -= 1

	//decrement H too if L is 0xFF
	if cpu.R.L == 0xFF {
		cpu.R.H -= 1
	}

}

//LDI r, (HL)
//Load the value from memory addressed in register pair (HL) and store it in register R. Increment the HL registers
func (cpu *Z80) LDIr_hl(r *byte) {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	*r = cpu.ReadByte(HL)

	//increment HL registers
	cpu.R.L += 1

	//increment H too if L is 0x00
	if cpu.R.L == 0x00 {
		cpu.R.H += 1
	}

	//set clock timings
}

//LDI (HL), r
//Load the value in register (r) and store in memory addressed in register pair (HL). Increment the HL registers
func (cpu *Z80) LDIhl_r(r *byte) {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	cpu.WriteByte(HL, *r)

	HL += 1

	cpu.R.H, cpu.R.L = utils.SplitIntoBytes(uint16(HL))
}

//LDH n, r
func (cpu *Z80) LDHn_r(r *byte) {
	var n byte = cpu.CurrentInstruction.Operands[0]
	cpu.WriteByte(types.Word(0xFF00)+types.Word(n), *r)
}

//LDH r, n
//Load value (n) in register (r) and store it in memory address FF00+PC. Increment PC by 1
func (cpu *Z80) LDHr_n(r *byte) {
	var n byte = cpu.CurrentInstruction.Operands[0]
	*r = cpu.ReadByte(types.Word(0xFF00) + types.Word(n))
}

//LD n, nn
func (cpu *Z80) LDn_nn(r1, r2 *byte) {
	var ls byte = cpu.CurrentInstruction.Operands[0]
	var hs byte = cpu.CurrentInstruction.Operands[1]

	//LS nibble first
	*r1 = hs
	*r2 = ls
}

//LD SP, nn
func (cpu *Z80) LDSP_nn() {
	var ls byte = cpu.CurrentInstruction.Operands[0]
	var hs byte = cpu.CurrentInstruction.Operands[1]

	var value types.Word = types.Word(utils.JoinBytes(hs, ls))
	cpu.SP = value
}

//LD SP, rr
func (cpu *Z80) LDSP_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	cpu.SP = HL
}

//LDHL SP, n 
func (cpu *Z80) LDHLSP_n() {
	var n types.Word = types.Word(cpu.CurrentInstruction.Operands[0])

	var HL types.Word
	if n > 127 {
		HL = cpu.SP - types.Word(-n)
	} else {
		HL = cpu.SP + types.Word(n)
	}

	cpu.R.H, cpu.R.L = utils.SplitIntoBytes(uint16(HL))

	//TODO: verify flag settings are correct....
	cpu.ResetFlag(Z)
	cpu.ResetFlag(N)

	//set carry flag
	if cpu.SP+n < cpu.SP {
		cpu.SetFlag(C)
	} else {
		cpu.ResetFlag(C)
	}

	//set half-carry flag
	if (((cpu.SP & 0xf) + (n & 0xf)) & 0x10) == 0x10 {
		cpu.SetFlag(H)
	} else {
		cpu.ResetFlag(H)
	}
}

//LD SP, n
func (cpu *Z80) LDnn_SP() {
	var nn types.Word = types.Word(utils.JoinBytes(cpu.CurrentInstruction.Operands[0], cpu.CurrentInstruction.Operands[1]))
	cpu.WriteWord(nn, cpu.SP)
}

//PUSH nn 
//Push register pair nn onto the stack and decrement the SP twice
func (cpu *Z80) Push_nn(r1, r2 *byte) {
	cpu.SP--
	cpu.WriteByte(cpu.SP, *r1)
	cpu.SP--
	cpu.WriteByte(cpu.SP, *r2)
}

//POP nn 
//Pop the stack twice onto register pair nn 
func (cpu *Z80) Pop_nn(r1, r2 *byte) {
	*r2 = cpu.ReadByte(cpu.SP)
	cpu.SP++
	*r1 = cpu.ReadByte(cpu.SP)
	cpu.SP++
}

//General add function
//side effect is flag alteration
func (cpu *Z80) add(a, b byte) byte {
	var calculation byte = a + b

	cpu.ResetFlag(N)

	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	if (calculation^b^a)&0x10 == 0x10 {
		cpu.SetFlag(H)
	} else {
		cpu.ResetFlag(H)
	}

	if calculation < a {
		cpu.SetFlag(C)
	} else {
		cpu.ResetFlag(C)
	}

	return calculation
}

//ADD A,r
//Add the value in register (r) to register A
func (cpu *Z80) AddA_r(r *byte) {
	cpu.R.A = cpu.add(cpu.R.A, *r)
}

//ADD A,(HL)
//Add the value in memory addressed in register pair (HL) to register A
func (cpu *Z80) AddA_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)

	cpu.R.A = cpu.add(cpu.R.A, value)
}

//ADD A,n
//Add the value in memory addressed PC to register A. Increment the PC by 1
func (cpu *Z80) AddA_n() {
	var value byte = cpu.CurrentInstruction.Operands[0]

	cpu.R.A = cpu.add(cpu.R.A, value)
}

//ADDC A,r
func (cpu *Z80) AddCA_r(r *byte) {
	var carryFlag byte = 0

	if cpu.IsFlagSet(C) {
		carryFlag = 1
	} else {
		carryFlag = 0
	}

	cpu.R.A = cpu.add(cpu.R.A, *r+carryFlag)
}

//ADDC A,(HL)
func (cpu *Z80) AddCA_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)
	var carryFlag byte = 0

	if cpu.IsFlagSet(C) {
		carryFlag = 1
	} else {
		carryFlag = 0
	}

	cpu.R.A = cpu.add(cpu.R.A, value+carryFlag)
}

//ADDC A,n
func (cpu *Z80) AddCA_n() {
	var value byte = cpu.CurrentInstruction.Operands[0]
	var carryFlag byte = 0

	if cpu.IsFlagSet(C) {
		carryFlag = 1
	} else {
		carryFlag = 0
	}

	cpu.R.A = cpu.add(cpu.R.A, value+carryFlag)
}

//General function for performing subtraction - will return value of calculation
//side effect is flags are set on the cpu accordingly
func (cpu *Z80) sub(a, b byte) byte {
	var calculation byte = a - b

	//set subtract flag
	cpu.SetFlag(N)

	//set zero flag if needed
	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	//Set Carry flag
	if calculation < a {
		cpu.SetFlag(C)
	} else {
		cpu.ResetFlag(C)
	}

	//Set half carry flag if needed
	if (calculation^b^a)&0x10 == 0x10 {
		cpu.SetFlag(H)
	} else {
		cpu.ResetFlag(H)
	}

	return calculation
}

//SUB A,r
func (cpu *Z80) SubA_r(r *byte) {
	cpu.R.A = cpu.sub(cpu.R.A, *r)
}

//SUB A,hl
func (cpu *Z80) SubA_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)

	cpu.R.A = cpu.sub(cpu.R.A, value)
}

//SUB A,n
func (cpu *Z80) SubA_n() {
	var value byte = cpu.CurrentInstruction.Operands[0]
	cpu.R.A = cpu.sub(cpu.R.A, value)
}

//SBC A,r
func (cpu *Z80) SubAC_r(r *byte) {
	var carryFlag byte = 0

	if cpu.IsFlagSet(C) {
		carryFlag = 1
	}

	cpu.R.A = cpu.sub(cpu.R.A, *r+carryFlag)
}

//SBC A, (HL)
func (cpu *Z80) SubAC_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)
	var carryFlag byte = 0

	if cpu.IsFlagSet(C) {
		carryFlag = 1
	}

	cpu.R.A = cpu.sub(cpu.R.A, value+carryFlag)
}

//SBC A, n
func (cpu *Z80) SubAC_n() {
	var value byte = cpu.CurrentInstruction.Operands[0]
	var carryFlag byte = 0

	if cpu.IsFlagSet(C) {
		carryFlag = 1
	}

	cpu.R.A = cpu.sub(cpu.R.A, value+carryFlag)
}

//Generic and function, returns calculation result
//and sets flags
func (cpu *Z80) and(a, b byte) byte {
	var calculation byte = a & b

	cpu.SetFlag(H)
	cpu.ResetFlag(N)
	cpu.ResetFlag(C)

	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	return calculation
}

//AND A, r
func (cpu *Z80) AndA_r(r *byte) {
	cpu.R.A = cpu.and(cpu.R.A, *r)
}

//AND A, (HL)
func (cpu *Z80) AndA_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)
	cpu.R.A = cpu.and(cpu.R.A, value)
}

//AND A, n
func (cpu *Z80) AndA_n() {
	var value byte = cpu.CurrentInstruction.Operands[0]
	cpu.R.A = cpu.and(cpu.R.A, value)
}

//calculates or, sets flag
func (cpu *Z80) or(a, b byte) byte {
	var calculation byte = a | b

	cpu.ResetFlag(H)
	cpu.ResetFlag(N)
	cpu.ResetFlag(C)

	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	return calculation
}

//OR A, r
func (cpu *Z80) OrA_r(r *byte) {
	cpu.R.A = cpu.or(cpu.R.A, *r)
}

//OR A, (HL)
func (cpu *Z80) OrA_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)
	cpu.R.A = cpu.or(cpu.R.A, value)
}

//OR A, n
func (cpu *Z80) OrA_n() {
	var value byte = cpu.CurrentInstruction.Operands[0]
	cpu.R.A = cpu.or(cpu.R.A, value)
}

//Generic xor function, sets flags as side effect
func (cpu *Z80) xor(a, b byte) byte {
	var calculation byte = a ^ b

	cpu.ResetFlag(H)
	cpu.ResetFlag(N)
	cpu.ResetFlag(C)

	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	return calculation
}

//XOR A, r
func (cpu *Z80) XorA_r(r *byte) {
	cpu.R.A = cpu.xor(cpu.R.A, *r)
}

//XOR A, (HL)
func (cpu *Z80) XorA_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)
	cpu.R.A = cpu.xor(cpu.R.A, value)
}

//XOR A, n
func (cpu *Z80) XorA_n() {
	var value byte = cpu.CurrentInstruction.Operands[0]
	cpu.R.A = cpu.xor(cpu.R.A, value)
}

//CP A, r
func (cpu *Z80) CPA_r(r *byte) {
	cpu.sub(cpu.R.A, *r)
}

//CP A, (HL) 
func (cpu *Z80) CPA_hl() {
	var hlAddr types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var hlValue byte = cpu.ReadByte(hlAddr)
	cpu.sub(cpu.R.A, hlValue)
}

//CP A, n
func (cpu *Z80) CPA_n() {
	var value byte = cpu.CurrentInstruction.Operands[0]
	cpu.sub(cpu.R.A, value)
}

//General increment function
//Side effect is CPU flags get set accordingly
func (cpu *Z80) inc(value byte) byte {
	var calculation byte = value + 0x01

	//N should be reset
	cpu.ResetFlag(N)

	//set zero flag
	if calculation == 0 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	//set half carry flag
	if (calculation^0x01^value)&0x10 == 0x10 {
		cpu.SetFlag(H)
	} else {
		cpu.ResetFlag(H)
	}

	return calculation
}

//INC r
func (cpu *Z80) Inc_r(r *byte) {
	*r = cpu.inc(*r)
}

//INC (HL)
func (cpu *Z80) Inc_hl() {
	var hlAddr types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var hlValue byte = cpu.ReadByte(hlAddr)
	var result byte = cpu.inc(hlValue)
	cpu.WriteByte(hlAddr, result)
}

//Generic dec function
func (cpu *Z80) dec(a byte) byte {
	var calculation byte = a - 1
	cpu.SetFlag(N)

	if calculation == 0 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	//Set half carry flag if needed
	if (calculation^0x01^a)&0x10 == 0x10 {
		cpu.SetFlag(H)
	} else {
		cpu.ResetFlag(H)
	}

	return calculation
}

//DEC r
func (cpu *Z80) Dec_r(r *byte) {
	*r = cpu.dec(*r)
}

//DEC (HL)
func (cpu *Z80) Dec_hl() {
	var hlAddr types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var hlValue byte = cpu.ReadByte(hlAddr)
	var result byte = cpu.dec(hlValue)
	cpu.WriteByte(hlAddr, result)
}

//ADD HL,rr
func (cpu *Z80) Addhl_rr(r1, r2 *byte) {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var oldHL types.Word = HL
	var RR types.Word = types.Word(utils.JoinBytes(*r1, *r2))
	HL += RR
	cpu.R.H, cpu.R.L = utils.SplitIntoBytes(uint16(HL))

	//reset N flag
	cpu.ResetFlag(N)

	//set carry flag
	if HL < oldHL {
		cpu.SetFlag(C)
	} else {
		cpu.ResetFlag(C)
	}

	//TODO Half Carry flag

}

//ADD HL,SP
func (cpu *Z80) Addhl_sp() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var oldHL types.Word = HL
	HL += cpu.SP
	cpu.R.H, cpu.R.L = utils.SplitIntoBytes(uint16(HL))

	//reset N flag
	cpu.ResetFlag(N)

	//set carry flag
	if HL < oldHL {
		cpu.SetFlag(C)
	} else {
		cpu.ResetFlag(C)
	}

	//TODO Half Carry flag
}

//ADD SP,n
func (cpu *Z80) Addsp_n() {
	var n byte = cpu.CurrentInstruction.Operands[0]

	//reset flags
	cpu.ResetFlag(Z)
	cpu.ResetFlag(N)

	var oldSP types.Word = cpu.SP

	// immediate value is signed
	if n > 127 {
		cpu.SP = cpu.SP - types.Word(-n)
	} else {
		cpu.SP += types.Word(n)
	}

	//check carry flag
	if cpu.SP < oldSP {
		cpu.SetFlag(C)
	} else {
		cpu.ResetFlag(C)
	}
	//TODO Half carry flag
}

//INC rr
func (cpu *Z80) Inc_rr(r1, r2 *byte) {
	var RR types.Word = types.Word(utils.JoinBytes(*r1, *r2))
	RR += 1
	*r1, *r2 = utils.SplitIntoBytes(uint16(RR))
}

//INC SP
func (cpu *Z80) Inc_sp() {
	cpu.SP += 1
}

//DEC rr
func (cpu *Z80) Dec_rr(r1, r2 *byte) {
	var RR types.Word = types.Word(utils.JoinBytes(*r1, *r2))
	RR -= 1
	*r1, *r2 = utils.SplitIntoBytes(uint16(RR))
}

//DEC SP
func (cpu *Z80) Dec_sp() {
	cpu.SP -= 1
}

//DAA
func (cpu *Z80) Daa() {
	//TODO: implement

	log.Fatalln(cpu.PC, cpu.CurrentInstruction, "Unimplemented")
}

//CPL
func (cpu *Z80) CPL() {
	cpu.R.A ^= 0xFF
	cpu.SetFlag(N)
	cpu.SetFlag(H)
}

//CCF
func (cpu *Z80) CCF() {

	if cpu.IsFlagSet(C) {
		cpu.ResetFlag(C)
	} else {
		cpu.SetFlag(C)
	}

	//Reset N and H flags
	cpu.ResetFlag(N)
	cpu.ResetFlag(H)

}

//SCF
func (cpu *Z80) SCF() {
	cpu.SetFlag(C)

	//Reset N and H flags
	cpu.ResetFlag(N)
	cpu.ResetFlag(H)
}

//generic swap
//flags will be affected by this
func (cpu *Z80) swap(a byte) byte {
	var calculation byte = utils.SwapNibbles(a)
	cpu.ResetFlag(N)
	cpu.ResetFlag(H)
	cpu.ResetFlag(C)

	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	return calculation
}

//SWAP r
func (cpu *Z80) Swap_r(r *byte) {
	*r = cpu.swap(*r)
}

//SWAP (HL)
func (cpu *Z80) Swap_hl() {
	var hlAddr types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var hlValue byte = cpu.ReadByte(hlAddr)
	var result = cpu.swap(hlValue)
	cpu.WriteByte(hlAddr, result)
}

func (cpu *Z80) rlc(value byte) byte {
	var bit7 bool = false

	if value&0x80 == 0x80 {
		bit7 = true
	}

	var calculation byte = value << 1

	if bit7 {
		cpu.SetFlag(C)
		calculation ^= 0x01
	} else {
		cpu.ResetFlag(C)
	}

	//zero flag
	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	//reset flags
	cpu.ResetFlag(N)
	cpu.ResetFlag(H)

	return calculation
}

//RLCA
func (cpu *Z80) RLCA() {
	cpu.R.A = cpu.rlc(cpu.R.A)
}

//RLC r
func (cpu *Z80) Rlc_r(r *byte) {
	*r = cpu.rlc(*r)
}

//RLC (HL)
func (cpu *Z80) Rlc_hl() {
	var hlAddr types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var hlValue byte = cpu.mmu.ReadByte(hlAddr)
	var result byte = cpu.rlc(hlValue)

	cpu.mmu.WriteByte(hlAddr, result)
}

func (cpu *Z80) rl_throughcarry(value byte) byte {
	var bit7 bool = false
	var calculation byte = value

	if calculation&0x80 == 0x80 {
		bit7 = true
	}

	calculation = calculation << 1

	if cpu.IsFlagSet(C) {
		calculation ^= 0x01
	}

	if bit7 {
		cpu.SetFlag(C)
	} else {
		cpu.ResetFlag(C)
	}

	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	cpu.ResetFlag(N)
	cpu.ResetFlag(H)

	return calculation
}

//RLA
func (cpu *Z80) RLA() {
	cpu.R.A = cpu.rl_throughcarry(cpu.R.A)
}

//RL r
func (cpu *Z80) Rl_r(r *byte) {
	*r = cpu.rl_throughcarry(*r)
}

//RL (HL)
func (cpu *Z80) Rl_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)
	var result byte = cpu.rl_throughcarry(value)
	cpu.WriteByte(HL, result)
}

func (cpu *Z80) rrc(value byte) byte {
	var bit0 bool = false

	if value&0x01 == 0x01 {
		bit0 = true
	}

	var calculation byte = value >> 1

	if bit0 {
		cpu.SetFlag(C)
		calculation ^= 0x80
	} else {
		cpu.ResetFlag(C)
	}

	//zero flag
	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	//reset flags
	cpu.ResetFlag(N)
	cpu.ResetFlag(H)

	return calculation
}

//RRCA
func (cpu *Z80) RRCA() {
	cpu.R.A = cpu.rrc(cpu.R.A)
}

//RRC r
func (cpu *Z80) Rrc_r(r *byte) {
	*r = cpu.rrc(*r)
}

//RRC (HL)
func (cpu *Z80) Rrc_hl() {
	var hlAddr types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var hlValue byte = cpu.mmu.ReadByte(hlAddr)
	var result byte = cpu.rrc(hlValue)

	cpu.mmu.WriteByte(hlAddr, result)
}

func (cpu *Z80) rr_throughcarry(value byte) byte {
	var bit0 bool = false
	var calculation byte = value

	if calculation&0x01 == 0x01 {
		bit0 = true
	}

	calculation = calculation >> 1

	if cpu.IsFlagSet(C) {
		calculation ^= 0x80
	}

	if bit0 {
		cpu.SetFlag(C)
	} else {
		cpu.ResetFlag(C)
	}

	if calculation == 0x00 {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

	cpu.ResetFlag(N)
	cpu.ResetFlag(H)

	return calculation
}

//RRA
func (cpu *Z80) RRA() {
	cpu.R.A = cpu.rr_throughcarry(cpu.R.A)
}

//RR r
func (cpu *Z80) Rr_r(r *byte) {
	*r = cpu.rr_throughcarry(*r)
}

//RR (HL)
func (cpu *Z80) Rr_hl() {
	var HLAddr types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HLAddr)
	var result byte = cpu.rr_throughcarry(value)

	cpu.WriteByte(HLAddr, result)
}

//BIT b, r
func (cpu *Z80) Bitb_r(b byte, r *byte) {

	cpu.ResetFlag(N)
	cpu.SetFlag(H)

	b = utils.BitToValue(b)

	if (*r & b) != b {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

}

//BIT b,(HL) 
func (cpu *Z80) Bitb_hl(b byte) {

	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var value byte = cpu.ReadByte(HL)

	cpu.ResetFlag(N)
	cpu.SetFlag(H)

	b = utils.BitToValue(b)

	if (value & b) != b {
		cpu.SetFlag(Z)
	} else {
		cpu.ResetFlag(Z)
	}

}

//NOP
//No operation
func (cpu *Z80) NOP() {
	//set clock values
}

//HALT
//Halt CPU
func (cpu *Z80) HALT() {
	cpu.Running = false
}

//STOP
func (cpu *Z80) Stop() {
	//TODO: Unimplemented
	log.Fatalln(cpu.PC, cpu.CurrentInstruction, "Unimplemented")
}

//DI
//Disable interrupts 
func (cpu *Z80) DI() {
	cpu.InterruptsEnabled = false
}

//EI
//Enable interrupts 
func (cpu *Z80) EI() {
	cpu.InterruptsEnabled = true
}

//JP nn
func (cpu *Z80) JP_nn() {
	var ls byte = cpu.CurrentInstruction.Operands[0]
	var hs byte = cpu.CurrentInstruction.Operands[1]
	cpu.PC = types.Word(utils.JoinBytes(hs, ls))
	cpu.PCJumped = true
}

//JP (HL)
func (cpu *Z80) JP_hl() {
	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var addr types.Word = cpu.ReadWord(HL)
	cpu.PC = addr
	cpu.PCJumped = true
}

//JP cc, nn
func (cpu *Z80) JPcc_nn(flag int, jumpWhen bool) {
	var ls byte = cpu.CurrentInstruction.Operands[0]
	var hs byte = cpu.CurrentInstruction.Operands[1]

	if cpu.IsFlagSet(flag) == jumpWhen {
		cpu.PCJumped = true
		cpu.PC = types.Word(utils.JoinBytes(hs, ls))
		cpu.LastInstrCycle.M = 4
	} else {
		cpu.LastInstrCycle.M = 3
	}
}

//JR n
func (cpu *Z80) JR_n() {
	var n byte = cpu.CurrentInstruction.Operands[0]

	cpu.PC += types.Word(cpu.CurrentInstruction.OperandsSize) + types.Word(1) //advance PC forward so we know how far to jump back

	if n > 127 {
		cpu.PCJumped = true
		cpu.PC -= types.Word(-n)
	} else {
		cpu.PCJumped = true
		cpu.PC += types.Word(n)
	}
}

//JR cc, nn
func (cpu *Z80) JRcc_nn(flag int, jumpWhen bool) {
	var n byte = cpu.CurrentInstruction.Operands[0]

	if cpu.IsFlagSet(flag) == jumpWhen {
		cpu.PC += types.Word(cpu.CurrentInstruction.OperandsSize + 1) //advance PC forward so we know how far to jump back
		if n > 127 {
			cpu.PCJumped = true
			cpu.PC -= types.Word(-n)
		} else {
			cpu.PCJumped = true
			cpu.PC += types.Word(n)
		}
		cpu.LastInstrCycle.M = 3
	} else {
		cpu.LastInstrCycle.M = 2
	}
}

// SET b, r
func (cpu *Z80) Setb_r(b byte, r *byte) {
	b = utils.BitToValue(b)

	*r = *r ^ b

	//set clock values
}

// SET b, (HL) 
func (cpu *Z80) Setb_hl(b byte) {
	b = utils.BitToValue(b)

	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var HLValue byte = cpu.ReadByte(HL)

	HLValue = HLValue ^ b

	cpu.WriteByte(HL, HLValue)
}

// RES b, r
func (cpu *Z80) Resb_r(b byte, r *byte) {

	b = utils.BitToValue(b)

	*r = *r &^ b

	//set clock values
}

// RES b, (HL) 
func (cpu *Z80) Resb_hl(b byte) {
	b = utils.BitToValue(b)

	var HL types.Word = types.Word(utils.JoinBytes(cpu.R.H, cpu.R.L))
	var HLValue byte = cpu.ReadByte(HL)

	HLValue = HLValue &^ b

	cpu.WriteByte(HL, HLValue)

	//set clock values
}

// CALL nn
//Push address of next instruction onto stack and then jump to address nn
func (cpu *Z80) Call_nn() {
	var ls byte = cpu.CurrentInstruction.Operands[0]
	var hs byte = cpu.CurrentInstruction.Operands[1]
	var nextInstr types.Word = cpu.PC + 3
	cpu.pushWordToStack(nextInstr)
	cpu.PC = types.Word(utils.JoinBytes(hs, ls))
	cpu.PCJumped = true
}

// CALL cc,nn
func (cpu *Z80) Callcc_nn(flag int, callWhen bool) {
	var ls byte = cpu.CurrentInstruction.Operands[0]
	var hs byte = cpu.CurrentInstruction.Operands[1]
	var nextInstr types.Word = cpu.PC + 3

	if cpu.IsFlagSet(flag) == callWhen {
		cpu.pushWordToStack(nextInstr)
		cpu.PC = types.Word(utils.JoinBytes(hs, ls))
		cpu.PCJumped = true
		cpu.LastInstrCycle.M = 6
	} else {
		cpu.LastInstrCycle.M = 3
	}
}

// RST n
func (cpu *Z80) Rst(n byte) {
	cpu.pushWordToStack(cpu.PC)
	cpu.PC = 0x0000 + types.Word(n)
	cpu.PCJumped = true
}

// RET
func (cpu *Z80) Ret() {
	cpu.PC = cpu.popWordFromStack()
	cpu.PCJumped = true
}

// RET cc
func (cpu *Z80) Retcc(flag int, returnWhen bool) {
	if cpu.IsFlagSet(flag) == returnWhen {
		cpu.PC = cpu.popWordFromStack()
		cpu.PCJumped = true
		cpu.LastInstrCycle.M = 5
	} else {
		cpu.LastInstrCycle.M = 2
	}
}

// RETI 
func (cpu *Z80) Ret_i() {
	cpu.PC = cpu.popWordFromStack()
	cpu.PCJumped = true

	cpu.InterruptsEnabled = true
}

//SLA r
func (cpu *Z80) Sla_r(r *byte) {
	//TODO: Implement

	log.Fatalln(cpu.PC, cpu.CurrentInstruction, "Unimplemented")
}

//SLA (HL) 
func (cpu *Z80) Sla_hl() {
	//TODO: Implement

	log.Fatalln(cpu.PC, cpu.CurrentInstruction, "Unimplemented")
}

//SRA r
func (cpu *Z80) Sra_r(r *byte) {
	//TODO: Implement

	log.Fatalln(cpu.PC, cpu.CurrentInstruction, "Unimplemented")
}

//SRA (HL) 
func (cpu *Z80) Sra_hl() {
	//TODO: Implement

	log.Fatalln(cpu.PC, cpu.CurrentInstruction, "Unimplemented")
}

//SRL r
func (cpu *Z80) Srl_r(r *byte) {
	//TODO: Implement

	log.Fatalln(cpu.PC, cpu.CurrentInstruction, "Unimplemented")
}

//SRL (HL) 
func (cpu *Z80) Srl_hl() {
	//TODO: Implement

	log.Fatalln(cpu.PC, cpu.CurrentInstruction, "Unimplemented")
}

//-----------------------------------------------------------------------
//INSTRUCTIONS END
