package main

import (
	"fmt"
	"os"

	"jvm/cmd"
)

// main 是程序的入口点
// 在 Go 中，main 函数是程序执行的起点
func main() {
	// 调用 cmd 包中的 Execute 函数来处理命令行参数
	// 这种设计模式将命令行逻辑分离到单独的包中，保持 main 函数简洁
	if err := cmd.Execute(); err != nil {
		// 如果执行过程中出现错误，打印错误信息并退出程序
		// fmt.Fprintf 允许我们将格式化的输出写入指定的 Writer（这里是 os.Stderr）
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		// os.Exit(1) 表示程序异常退出，返回码为 1
		os.Exit(1)
	}
}
