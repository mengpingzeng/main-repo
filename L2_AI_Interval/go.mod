module claw_studios/L2_AI_Interval

go 1.24.11

require (
	github.com/go-sql-driver/mysql v1.10.0
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/prometheus/client_golang v1.19.0
	github.com/robfig/cron/v3 v3.0.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	L0_AI_Account_Secret_Vault v0.0.0
	clawstudios/l1_ai_releaser v0.0.0
	clawstudios/pkg/logging v0.0.0
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.48.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
)

replace clawstudios/l1_ai_releaser => /home/zmp/claw_studios/L1_AI_Releaser

replace L0_AI_Account_Secret_Vault => /home/zmp/claw_studios/L0_AI_Account_Secret_Vault

replace clawstudios/pkg/logging => /home/claw_studios/code/pkg/logging
