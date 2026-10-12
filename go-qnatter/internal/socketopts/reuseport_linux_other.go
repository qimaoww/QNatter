//go:build linux && !(mips || mipsle || mips64 || mips64le)

package socketopts

// soReusePort 是 Linux（非 MIPS）上的 SO_REUSEPORT 值。
const soReusePort = 0x0f
