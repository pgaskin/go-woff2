# go-woff2

[![Go Reference](https://pkg.go.dev/badge/github.com/pgaskin/go-woff2.svg)](https://pkg.go.dev/github.com/pgaskin/go-woff2)
[![Test](https://github.com/pgaskin/go-woff2/actions/workflows/test.yml/badge.svg)](https://github.com/pgaskin/go-woff2/actions/workflows/test.yml)
[![Attest woff2 build](https://github.com/pgaskin/go-woff2/actions/workflows/attest.yml/badge.svg)](https://github.com/pgaskin/go-woff2/actions/workflows/attest.yml)

Go bindings for woff2 without cgo.

This library wraps a WebAssembly build of [woff2](https://github.com/google/woff2) transpiled to Go using [wasm2go](https://github.com/ncruces/wasm2go).

The wasm2go blob is fully [reproducible](./src/Dockerfile) and [verified](https://github.com/pgaskin/go-woff2/attestations).

To have working IDE integration while working on the bindings, use `bear -- make -C src distclean download all CC=/path/to/wasi-sdk/bin/wasm32-wasip1-clang CXX=/path/to/wasi-sdk/bin/wasm32-wasip1-clang++ WASM_OPT=/path/to/binaryen/bin/wasm-opt` to download the woff2/brotli source and generate the `compile_commands.json`.
