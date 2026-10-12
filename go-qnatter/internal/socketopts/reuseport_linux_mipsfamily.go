//go:build linux && (mips || mipsle || mips64 || mips64le)

package socketopts

// soReusePort 在 Linux/MIPS 上的取值与其他架构不同，硬编码 0x0f 会静默
// 设置成错误的套接字选项。
const soReusePort = 0x200
