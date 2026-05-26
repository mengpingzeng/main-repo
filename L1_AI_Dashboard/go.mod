module L1_AI_Dashboard

go 1.24.0

toolchain go1.24.11

require (
	clawstudios/pkg/logging v0.0.0
	github.com/go-sql-driver/mysql v1.10.0
	github.com/mattn/go-sqlite3 v1.14.44
)

replace clawstudios/pkg/logging => /home/claw_studios/code/pkg/logging

require filippo.io/edwards25519 v1.2.0 // indirect
