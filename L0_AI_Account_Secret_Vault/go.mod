module L0_AI_Account_Secret_Vault

go 1.25.0

require (
	clawstudios/pkg/logging v0.0.0
	github.com/go-sql-driver/mysql v1.10.0
	golang.org/x/crypto v0.51.0
)

replace clawstudios/pkg/logging => /home/claw_studios/code/pkg/logging

require filippo.io/edwards25519 v1.2.0 // indirect
