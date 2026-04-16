package libcore

// gomobile bind 需要 golang.org/x/mobile/bind 包可用, 但我们代码不直接引用它。
// 没有这个 blank import, `go mod tidy` 会把它清出 go.mod, 然后 gomobile bind 失败。
// 详见 CLAUDE.md Known Issue #M10。
import _ "golang.org/x/mobile/bind"
