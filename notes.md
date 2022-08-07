# 介绍

1. 介绍什么是虚拟机与编译器
- 物理机：CPU，内存，输入输出设备 ==> 寻址执行
- 虚拟机：输入byte code、执行、得到结果
- 编译器：将一种代码翻译成另外一种更底层代码，并做一些代码优化。

虚拟机从架构上可以分为两大类：
- register virtual machine
- stack virtual machine

stack VM更加简单，概念也更少。

整体流程： lexer -> parser -> compiler -> virtual machine

数据结构： source code -> ast -> byte code -> objects

为什么要有中间表示（IR），为什么用byte code？

内存格式：byte code, opcode operands, 大小端序(big-endian, little-endian)

call stack 调用堆栈

virtual machine 与 compiler 的对偶关系






