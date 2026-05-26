module L2_AI_Workflow_Engine

go 1.24.11

require (
	L0_AI_Account_Secret_Vault v0.0.0
	a4md v0.0.0
	clawstudios/l1_ai_releaser v0.0.0
	clawstudios/pkg/logging v0.0.0
	github.com/go-sql-driver/mysql v1.10.0
	github.com/mattn/go-sqlite3 v1.14.44
)

replace clawstudios/pkg/logging => /home/claw_studios/code/pkg/logging

require filippo.io/edwards25519 v1.2.0 // indirect

replace clawstudios/l1_ai_releaser => /home/claw_studios/code/L1_AI_Releaser

replace L0_AI_Account_Secret_Vault => /home/claw_studios/code/L0_AI_Account_Secret_Vault

replace a4md => /home/claw_studios/code/L1_AI_Doc_Hub
