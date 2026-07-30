# Running Tailscale on WSL1

WSL1 (kernel `4.4.0-*-Microsoft`) emulates Linux syscalls on the NT kernel
and needs two deviations from a stock Linux deployment of Tailscale.

## tailscaled must be built with cgo

Symptom of a non-cgo build: every Tailscale SSH command execution
(`ssh host cmd`, scp, sftp) hangs, and a `tailscaled be-child ssh ...`
process spins at 100% CPU until the client disconnects. Interactive login
shells still work, because they go through `/bin/login`, which changes
users on its own.

Cause: WSL1's `tgkill` is missing real Linux's same-thread-group exception
in its signal permission check. The SSH incubator drops privileges with
Go's `syscall.Setuid`, which without cgo applies the syscall on the calling
thread first and then signals every other OS thread (`AllThreadsSyscall`).
After the first thread transitions root -> user, WSL1 rejects the signals
to the remaining root threads with EPERM; the Go runtime ignores the
error and spin-waits forever. glibc's setuid signals sibling threads
*before* transitioning, so a cgo build — which dispatches `Setuid` to
glibc — works.

Build both binaries release-style, with only cgo flipped on:

    TS_USE_TOOLCHAIN=1 CGO_ENABLED=1 ./build_dist.sh tailscale.com/cmd/tailscaled
    TS_USE_TOOLCHAIN=1 CGO_ENABLED=1 ./build_dist.sh tailscale.com/cmd/tailscale

Notes:

- The resulting binaries are dynamically linked against glibc, so build
  them on (or for) the machine that runs them.
- cgo needs the Linux UAPI headers (`/usr/include/linux`); on Slackware
  install the `kernel-headers` package first.
- WSL1 has no TUN device; run tailscaled with
  `--tun=userspace-networking`.

## TS_OS_SPOOF_WINDOWS

`version.OS()` reports `windows` on Linux only when
`TS_OS_SPOOF_WINDOWS=1` is set in tailscaled's environment. This exists
for the dual-boot state-sharing setup, where a Linux install reuses the
state written by a Windows Tailscale and should keep presenting itself as
that Windows node.

Do not set it on an established Linux node: the coordination server flags
the OS change ("node OS changed since last connection, was node state
copied between devices?") and serves the node an empty peer list and an
empty Tailscale SSH policy, which disables SSH access entirely.
