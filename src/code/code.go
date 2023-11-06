package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Opcode byte
type Instructions []byte // byte code 里面第一个byte是opcode，后续是操作数

const (
	OpConstant Opcode = iota
	OpAdd
	OpSub
	OpMul
	OpDiv

	OpPop

	OpTrue
	OpFalse

	OpEqual
	OpNotEqual
	OpGreaterThan

	OpMinus
	OpBang

	OpJumpNotTruthy
	OpJump
	OpNull

	OpGetGlobal
	OpSetGlobal

	OpArray
	OpHash
	OpIndex

	OpCall
	OpReturnValue
	OpReturn
	OpSetLocal
	OpGetLocal

	OpGetBuiltin

	OpClosure
)

type Definition struct {
	Name          string
	OperandWidths []int // 一个操作符可能有多个操作数，每个操作数占几个byte
}

var definitions = map[Opcode]*Definition{
	OpConstant: {"OpConstant", []int{2}},

	OpAdd: {"OpAdd", []int{}},
	OpPop: {"OpPop", []int{}},

	OpSub: {"OpSub", []int{}},
	OpMul: {"OpMul", []int{}},
	OpDiv: {"OpDiv", []int{}},

	OpTrue:  {"OpTrue", []int{}},
	OpFalse: {"OpFalse", []int{}},

	OpEqual:       {"OpEqual", []int{}},
	OpNotEqual:    {"OpNotEqual", []int{}},
	OpGreaterThan: {"OpGreaterThan", []int{}},

	OpMinus: {"OpMinus", []int{}},
	OpBang:  {"OpBang", []int{}},

	OpJumpNotTruthy: {"OpJumpNotTruthy", []int{2}},
	OpJump:          {"OpJump", []int{2}},

	OpNull: {"OpNull", []int{}},

	OpGetGlobal: {"OpGetGlobal", []int{2}},
	OpSetGlobal: {"OpSetGlobal", []int{2}},

	OpArray: {"OpArray", []int{2}},
	OpHash:  {"OpHash", []int{2}},
	OpIndex: {"OpIndex", []int{}},

	// 参数的个数，可以用来在object stack上跳过参数，寻找function object。
	OpCall: {"OpCall", []int{1}},

	OpReturnValue: {"OpReturnValue", []int{}},
	OpReturn:      {"OpReturn", []int{}},

	OpSetLocal: {"OpSetLocal", []int{1}},
	OpGetLocal: {"OpGetLocal", []int{1}},

	// 只有一个操作数，就是内置函数slice的下标
	OpGetBuiltin: {"OpGetBuiltin", []int{1}},

	// 2 byte-wide: the index of function in constant pool
	// 1 byte-wide: free variable index
	OpClosure: {"OpClosure", []int{2, 1}},
}

func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}

	return def, nil
}

func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	instructionLen := 1
	for _, w := range def.OperandWidths {
		instructionLen += w
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)
	offset := 1
	for i, operand := range operands {
		width := def.OperandWidths[i]

		switch width {
		case 2:
			binary.BigEndian.PutUint16(instruction[offset:], uint16(operand))
		case 1:
			instruction[offset] = byte(operand)
		}
		offset += width
	}

	return instruction
}

func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR:%s\n", err)
			continue
		}

		// 针对一个opcode，读取对应的操作数
		operands, read := ReadOperands(def, ins[i+1:])
		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstruction(def, operands))

		// 下一个操作符位置 = 当前操作符位置 + 1 + 读取的操作数bytes数
		i += 1 + read
	}

	return out.String()
}

func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {
	operandCount := len(def.OperandWidths)
	if len(operands) != operandCount {
		return fmt.Sprintf("ERROR: operand len %d does not match defined %d\n", len(operands), operandCount)
	}

	switch operandCount {
	case 0:
		return def.Name
	case 1:
		return fmt.Sprintf("%s %d", def.Name, operands[0])
	case 2:
		return fmt.Sprintf("%s %d %d", def.Name, operands[0], operands[1])
	}

	return fmt.Sprintf("ERROR: unhandled operandCount for %s\n", def.Name)
}

func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))
		case 1:
			operands[i] = int(ReadUint8(ins[offset:]))
		}

		offset += width
	}
	return operands, offset
}

func ReadUint16(ins Instructions) uint16 {
	return binary.BigEndian.Uint16(ins)
}

func ReadUint8(ins Instructions) uint8 {
	return uint8(ins[0])
}
