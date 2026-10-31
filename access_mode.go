//go:build !server

package main

// accessModeServer is the compile-time mode switch behind the three-tier
// access design (plan §3.4). A plain desktop build (`go build .`) serves a
// single user who is always admin, so the constant is false here and
// effectiveRole() short-circuits to RoleAdmin with zero behaviour change.
// The server build (`go build -tags server`, see build/Taskfile.yml) compiles
// access_mode_server.go instead, where the constant is true and the effective
// role comes from config.json / WGIT_ROLE.
const accessModeServer = false
