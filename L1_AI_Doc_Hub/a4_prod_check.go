//go:build ignore

// A4 文档沉淀模块 - 生产环境自测脚本
//
// 用法：go run ./cmd/prod_check
//
// 本文件加 build ignore 是因为 Go 不允许同目录下存在 main 和 a4md 两个 package。
// 实际自测入口是 cmd/prod_check/main.go。
package main

import "fmt"

func main() {
	fmt.Println("请使用: go run ./cmd/prod_check")
}
