package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// GetProjectRoot 通过查找go.mod文件来获取项目根目录
// 这种方法比依赖文件层级更可靠
func GetProjectRoot() (string, error) {
	// 尝试通过go.mod文件定位项目根目录
	modDir, err := findGoModDir("")
	if err == nil {
		return modDir, nil
	}

	// 如果找不到go.mod，则使用备用方案：通过当前文件路径推断
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("无法获取当前文件路径: %w", err)
	}

	// 根据项目结构，当前文件在utils/目录下，向上回退1级到项目根目录
	// 如果这个文件在其他位置，调整这里的层级
	projectRoot := filepath.Join(filepath.Dir(filename), "..")

	absPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}

	// 验证是否是有效的项目根目录（检查是否存在main.go或其他标志性文件）
	if isValidProjectRoot(absPath) {
		return absPath, nil
	}

	// 尝试从当前文件路径向上查找main.go
	return findMainGoDir(filepath.Dir(filename))
}

// findGoModDir 从指定目录开始向上查找包含go.mod文件的目录
func findGoModDir(startDir string) (string, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	currentDir := absDir
	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if fileExists(goModPath) {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}

		currentDir = parentDir
	}

	return "", fmt.Errorf("未找到go.mod文件")
}

// findMainGoDir 向上查找包含main.go的目录
func findMainGoDir(startDir string) (string, error) {
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	currentDir := absDir
	for {
		mainGoPath := filepath.Join(currentDir, "main.go")
		if fileExists(mainGoPath) {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}

		currentDir = parentDir
	}

	return "", fmt.Errorf("未找到包含main.go的目录")
}

// isValidProjectRoot 检查目录是否看起来像项目根目录
func isValidProjectRoot(path string) bool {
	// 检查常见的项目根目录文件/目录
	commonRootFiles := []string{
		"main.go",
		"go.mod",
		"go.sum",
		".git",
	}

	for _, file := range commonRootFiles {
		if fileExists(filepath.Join(path, file)) {
			return true
		}
	}

	return false
}

// fileExists 检查文件或目录是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}