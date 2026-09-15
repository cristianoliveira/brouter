# Vendored Lua 5.4.7

- Source: https://www.lua.org/ftp/lua-5.4.7.tar.gz
- sha256: 9fbf5e28ef86c69858f6d3d34eccc32e911c1a28b4120ff3e84aaa70cfbf1e30
- License: MIT (see LICENSE in this directory)
- Purpose: the isolated route-script helper (`native/lua-helper`) only.
  The Go router remains CGO-free; this interpreter never links into any
  Go binary. The helper opens base/table/string only and never
  registers os/io/load/require — see the contract in
  docs/route-script-contract.md.
