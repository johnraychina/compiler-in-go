package vm

import (
	"fmt"
	"monkey/code"
	"monkey/compiler"
	"monkey/object"
)

const StackSize = 2048
const GlobalSize = 65535

// 预分配frame，一个是避免slice指针变化，简化代码处理，另一方面提高性能。
const MaxFrames = 1024

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

type VM struct {
	constants []object.Object

	stack []object.Object
	sp    int // always points to the next value. Top of stack is stack[sp-1]

	globals []object.Object

	frames     []*Frame
	frameIndex int
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.frameIndex-1]
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.frameIndex] = f
	vm.frameIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.frameIndex--
	return vm.frames[vm.frameIndex]
}

type Frame struct {
	fn *object.CompiledFunction
	ip int
}

func NewFrame(fn *object.CompiledFunction) *Frame {
	return &Frame{fn: fn, ip: -1}
}

func (f *Frame) Instructions() code.Instructions {
	return f.fn.Instructions
}

func NewWithGlobalsStore(bytecode *compiler.Bytecode, s []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = s
	return vm
}

func New(bytecode *compiler.Bytecode) *VM {
	// main函数也作为一个function，用frame封装起来。
	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainFrame := NewFrame(mainFn)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants: bytecode.Constants,

		stack: make([]object.Object, StackSize),
		sp:    0,

		globals:    make([]object.Object, GlobalSize),
		frames:     frames,
		frameIndex: 1, // frameIndex 0 已经被mainFrame占用
	}
}

// Run fetch-decode-execute
func (vm *VM) Run() error {
	var ip int
	var ins code.Instructions
	var op code.Opcode

	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {

		vm.currentFrame().ip++
		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()
		op = code.Opcode(ins[ip])

		switch op {
		case code.OpJump:
			jumpPos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip = jumpPos - 1 // 跳转到指定位置前面的那个byte
		case code.OpJumpNotTruthy:
			jumpPos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			condition := vm.pop()
			if !isTruthy(condition) {
				vm.currentFrame().ip = jumpPos - 1
			}
		case code.OpNull:
			err := vm.push(Null)
			if err != nil {
				return err
			}
		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}
		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}
		case code.OpPop:
			vm.pop()
		case code.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}
		case code.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}
		case code.OpGreaterThan, code.OpNotEqual, code.OpEqual:
			err := vm.executeComparison(op)
			if err != nil {
				return err
			}
		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil {
				return err
			}
		case code.OpMinus:
			err := vm.executeMinusOperator()
			if err != nil {
				return err
			}
		case code.OpSetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2
			vm.globals[globalIndex] = vm.pop()

		case code.OpGetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.globals[globalIndex])
			if err != nil {
				return err
			}
		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.sp = vm.sp - numElements

			err := vm.push(array)
			if err != nil {
				return err
			}
		case code.OpHash:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			hash, err := vm.buildHash(vm.sp-numElements, vm.sp)
			if err != nil {
				return err
			}
			vm.sp = vm.sp - numElements

			err = vm.push(hash)
			if err != nil {
				return err
			}
		case code.OpIndex:
			index := vm.pop()
			left := vm.pop()

			err := vm.executeIndexExpression(left, index)
			if err != nil {
				return err
			}
		case code.OpCall:
			// 前面已经通过OpGetGlobal将函数放到vm 操作数stack上了，这里调用函数
			fn, ok := vm.stack[vm.sp-1].(*object.CompiledFunction)
			if !ok {
				return fmt.Errorf("calling non-function")
			}

			// 把函数放到一个新的frame作为current frame（在main frame上面）
			// 到下一个循环时，就会取对应新的frame的指令执行了
			frame := NewFrame(fn)
			vm.pushFrame(frame)
		case code.OpReturnValue:
			// 当前frame执行结果从操作数栈取出
			returnVal := vm.pop()
			// 弹出已经执行完成的function frame
			vm.popFrame()
			vm.pop()

			// 将function执行结果放回操作数栈
			err := vm.push(returnVal)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	default:
		return true
	}
}

func (vm *VM) executeIndexExpression(left object.Object, index object.Object) error {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return vm.executeArrayIndex(left, index)
	case left.Type() == object.HASH_OBJ:
		return vm.exeucteHashIndex(left, index)
	default:
		return fmt.Errorf("index operator is not supported: %s", left.Type())
	}
}

func (vm *VM) executeArrayIndex(left object.Object, index object.Object) error {
	array := left.(*object.Array)
	i := index.(*object.Integer).Value
	max := int64(len(array.Elements) - 1)

	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(array.Elements[i])
}

func (vm *VM) exeucteHashIndex(left object.Object, index object.Object) error {
	hashObj := left.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObj.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}
	return vm.push(pair.Value)
}

func (vm *VM) executeBangOperator() error {
	operand := vm.pop()
	switch operand {
	case True:
		return vm.push(False)
	case False:
		return vm.push(True)
	case Null:
		return vm.push(True)
	default:
		return vm.push(False)
	}
}

func (vm *VM) executeMinusOperator() error {
	operand := vm.pop()
	if operand.Type() != object.INTEGER_OBJ {
		return fmt.Errorf("unsupported type for negation: %s", operand.Type())
	}
	val := operand.(*object.Integer).Value
	return vm.push(&object.Integer{Value: -val})
}

func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	leftType := left.Type()
	rightType := right.Type()

	switch {
	case leftType == object.INTEGER_OBJ && rightType == object.INTEGER_OBJ:
		return vm.executeBinaryIntegerOperation(op, left, right)
	case leftType == object.STRING_OBJ && rightType == object.STRING_OBJ:
		return vm.executeBinaryStringOperation(op, left, right)
	default:
		return fmt.Errorf("unsupported types for binary operation: %s %s", leftType, rightType)
	}
}

func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparison(op, left, right)
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBoolean(right == left))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBoolean(right != left))
	default:
		return fmt.Errorf("unknown operator: %d (%s %s)", op, left.Type(), right.Type())
	}
}

func (vm *VM) executeIntegerComparison(op code.Opcode, left object.Object, right object.Object) error {
	lv := left.(*object.Integer).Value
	rv := right.(*object.Integer).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBoolean(lv == rv))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBoolean(lv != rv))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBoolean(lv > rv))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func nativeBoolToBoolean(b bool) object.Object {
	if b {
		return True
	}
	return False
}

func (vm *VM) executeBinaryStringOperation(op code.Opcode, left, right object.Object) error {

	if op != code.OpAdd {
		return fmt.Errorf("unknown string operator: %d", op)
	}
	rightValue := right.(*object.String).Value
	leftValue := left.(*object.String).Value

	return vm.push(&object.String{Value: leftValue + rightValue})
}

func (vm *VM) executeBinaryIntegerOperation(op code.Opcode, left, right object.Object) error {
	rightValue := right.(*object.Integer).Value
	leftValue := left.(*object.Integer).Value

	var result int64

	// todo overflow check
	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		result = leftValue / rightValue
	}

	return vm.push(&object.Integer{Value: result})
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}

	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) buildArray(spStart int, spEnd int) *object.Array {
	elements := make([]object.Object, spEnd-spStart)

	for i := spStart; i < spEnd; i++ {
		elements[i-spStart] = vm.stack[i]
	}

	return &object.Array{Elements: elements}
}

func (vm *VM) buildHash(spStart int, spEnd int) (*object.Hash, error) {
	pairs := make(map[object.HashKey]object.HashPair)

	for i := spStart; i < spEnd; i += 2 {

		k := vm.stack[i]
		v := vm.stack[i+1]
		pair := object.HashPair{Key: k, Value: v}

		hashKey, ok := k.(object.Hashable)
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", k.Type())
		}

		pairs[hashKey.HashKey()] = pair
	}

	return &object.Hash{Pairs: pairs}, nil
}
