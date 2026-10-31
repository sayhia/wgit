//go:build server

package main

// accessModeServer is the compile-time mode switch behind the three-tier
// access design (plan §3.4). The server build (`go build -tags server`, see
// build/Taskfile.yml) serves shared deployments where viewers and operators
// connect with reduced rights, so effectiveRole() consults the access tier
// configured in config.json (AppConfig.Access) with the WGIT_ROLE environment
// override. A plain desktop build compiles access_mode.go instead, where the
// constant is false and every caller is admin.
const accessModeServer = true
