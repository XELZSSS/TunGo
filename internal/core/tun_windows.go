package core

import (
	"fmt"
	"os/exec"

	"tungo/internal/spawn"
)

func resetTun() {
	script := fmt.Sprintf(`$ErrorActionPreference='SilentlyContinue'
Get-PnpDevice -Class Net -ErrorAction SilentlyContinue | Where-Object {
  $_.FriendlyName -like '*%s*' -and (-not $_.Present -or $_.Status -ne 'OK')
} | ForEach-Object { & pnputil /remove-device $_.InstanceId | Out-Null }
`, tunInterface)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	spawn.Hide(cmd)
	cmd.Run()
}
